import Foundation
import Combine
import SwiftUI

/// Main ViewModel for the WhatsApp app
@MainActor
class ChatViewModel: ObservableObject {
    // MARK: - Published Properties

    // Connection state
    @Published var isConnecting = false
    @Published var isConnected = false
    @Published var isLoggedIn = false
    @Published var currentQRCode: String?
    @Published var connectionError: String?

    // User info
    @Published var myJID: String?
    @Published var myName: String = ""
    @Published var myPhoneNumber: String = ""

    // Data
    @Published var chats: [Chat] = []
    @Published var contacts: [Contact] = []
    @Published var messages: [String: [Message]] = [:] // chatJID -> messages
    @Published var selectedChat: Chat?

    // UI state
    @Published var isLoadingChats = false
    @Published var isLoadingMessages = false
    @Published var searchText = ""

    // MARK: - Private Properties

    private let client = WhatsAppClient.shared
    private var cancellables = Set<AnyCancellable>()

    // MARK: - Computed Properties

    var filteredChats: [Chat] {
        if searchText.isEmpty {
            return sortedChats
        }
        return sortedChats.filter {
            $0.name.localizedCaseInsensitiveContains(searchText) ||
            $0.lastMessage.localizedCaseInsensitiveContains(searchText)
        }
    }

    var sortedChats: [Chat] {
        chats.sorted { chat1, chat2 in
            // Pinned chats first
            if chat1.isPinned != chat2.isPinned {
                return chat1.isPinned
            }
            // Then by last message time
            return chat1.lastMessageTime > chat2.lastMessageTime
        }
    }

    var pinnedChats: [Chat] {
        filteredChats.filter { $0.isPinned }
    }

    var regularChats: [Chat] {
        filteredChats.filter { !$0.isPinned }
    }

    var unreadCount: Int {
        chats.reduce(0) { $0 + $1.unreadCount }
    }

    // MARK: - Initialization

    init() {
        setupBindings()
    }

    private func setupBindings() {
        // Observe client state
        client.$isConnected
            .receive(on: DispatchQueue.main)
            .assign(to: &$isConnected)

        client.$isLoggedIn
            .receive(on: DispatchQueue.main)
            .assign(to: &$isLoggedIn)

        client.$currentQRCode
            .receive(on: DispatchQueue.main)
            .assign(to: &$currentQRCode)

        client.$myJID
            .receive(on: DispatchQueue.main)
            .assign(to: &$myJID)

        client.$connectionError
            .receive(on: DispatchQueue.main)
            .assign(to: &$connectionError)
    }

    // MARK: - Connection

    func initialize() {
        client.initialize()

        // Check if we have stored credentials
        if UserDefaults.standard.bool(forKey: .isLoggedIn) {
            connect()
        }
    }

    func connect() {
        isConnecting = true
        connectionError = nil
        client.connect()

        // For demo, simulate connection after delay
        Task {
            try? await Task.sleep(nanoseconds: 2_000_000_000)
            isConnecting = false
        }
    }

    func disconnect() {
        client.disconnect()
        isConnected = false
    }

    func logout() async {
        do {
            try await client.logout()
            isLoggedIn = false
            isConnected = false
            myJID = nil
            chats = []
            contacts = []
            messages = [:]
            UserDefaults.standard.set(false, forKey: .isLoggedIn)
        } catch {
            connectionError = error.localizedDescription
        }
    }

    // MARK: - Demo Login (for testing without real WhatsApp)

    func demoLogin() {
        isConnecting = true

        Task {
            try? await Task.sleep(nanoseconds: 1_500_000_000)

            isConnecting = false
            isLoggedIn = true
            isConnected = true
            currentQRCode = nil
            myJID = "1234567890@s.whatsapp.net"
            myName = "Demo User"
            myPhoneNumber = "+1 (234) 567-890"

            // Load demo data
            loadDemoData()

            UserDefaults.standard.set(true, forKey: .isLoggedIn)
        }
    }

    private func loadDemoData() {
        chats = Chat.samples
        contacts = Contact.samples

        // Add some demo messages
        for chat in chats {
            messages[chat.jid] = generateDemoMessages(for: chat)
        }
    }

    private func generateDemoMessages(for chat: Chat) -> [Message] {
        let names = ["Alice", "Bob", "Carol", "David"]
        let texts = [
            "Hey, how are you?",
            "I'm doing great, thanks!",
            "Did you see the news?",
            "Let's meet up tomorrow",
            "Sounds good!",
            "See you then!",
        ]

        var demoMessages: [Message] = []
        let count = Int.random(in: 5...15)

        for i in 0..<count {
            let isFromMe = Bool.random()
            let message = Message(
                id: UUID().uuidString,
                chatJID: chat.jid,
                senderJID: isFromMe ? (myJID ?? "") : chat.jid,
                senderName: isFromMe ? "Me" : (chat.isGroup ? names.randomElement()! : chat.name),
                text: texts.randomElement()!,
                timestamp: Date().addingTimeInterval(-Double(count - i) * 3600),
                isFromMe: isFromMe,
                isGroup: chat.isGroup,
                status: isFromMe ? .read : .delivered
            )
            demoMessages.append(message)
        }

        return demoMessages.sorted { $0.timestamp < $1.timestamp }
    }

    // MARK: - Messaging

    func sendMessage(to chatJID: String, text: String) async {
        guard !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return }

        // Create local message immediately for responsiveness
        let localMessage = Message(
            id: UUID().uuidString,
            chatJID: chatJID,
            senderJID: myJID ?? "",
            senderName: "Me",
            text: text,
            timestamp: Date(),
            isFromMe: true,
            isGroup: chatJID.isGroupJID,
            status: .sending
        )

        // Add to messages
        if messages[chatJID] == nil {
            messages[chatJID] = []
        }
        messages[chatJID]?.append(localMessage)

        // Update chat's last message
        if let index = chats.firstIndex(where: { $0.jid == chatJID }) {
            chats[index].lastMessage = text
            chats[index].lastMessageTime = Date()
        }

        do {
            // Send via client
            let messageID = try await client.sendTextMessage(to: chatJID, text: text)

            // Update message status
            if let msgIndex = messages[chatJID]?.firstIndex(where: { $0.id == localMessage.id }) {
                messages[chatJID]?[msgIndex].status = .sent
                // Update ID with server-assigned ID
                var updatedMessage = messages[chatJID]![msgIndex]
                updatedMessage = Message(
                    id: messageID,
                    chatJID: updatedMessage.chatJID,
                    senderJID: updatedMessage.senderJID,
                    senderName: updatedMessage.senderName,
                    text: updatedMessage.text,
                    timestamp: updatedMessage.timestamp,
                    isFromMe: updatedMessage.isFromMe,
                    isGroup: updatedMessage.isGroup,
                    status: .sent
                )
                messages[chatJID]?[msgIndex] = updatedMessage
            }

            HapticFeedback.light()
        } catch {
            // Mark as failed
            if let msgIndex = messages[chatJID]?.firstIndex(where: { $0.id == localMessage.id }) {
                messages[chatJID]?[msgIndex].status = .failed
            }
            HapticFeedback.error()
        }
    }

    func sendImage(to chatJID: String, imageData: Data, caption: String = "") async {
        do {
            _ = try await client.sendImageMessage(to: chatJID, imageData: imageData, caption: caption)
            HapticFeedback.success()
        } catch {
            connectionError = error.localizedDescription
            HapticFeedback.error()
        }
    }

    func markAsRead(chatJID: String) async {
        guard let chatMessages = messages[chatJID] else { return }

        let unreadMessageIDs = chatMessages
            .filter { !$0.isFromMe && $0.status != .read }
            .map { $0.id }

        guard !unreadMessageIDs.isEmpty else { return }

        do {
            try await client.markAsRead(chatJID: chatJID, messageIDs: unreadMessageIDs)

            // Update local state
            if let index = chats.firstIndex(where: { $0.jid == chatJID }) {
                chats[index].unreadCount = 0
            }
        } catch {
            print("Failed to mark as read: \(error)")
        }
    }

    func sendTyping(to chatJID: String, isTyping: Bool) {
        Task {
            try? await client.sendTyping(to: chatJID, isTyping: isTyping)
        }
    }

    // MARK: - Chat Management

    func loadMessages(for chatJID: String) async {
        isLoadingMessages = true

        // For demo, messages are already loaded
        // In production, fetch from database or server

        isLoadingMessages = false
    }

    func getMessages(for chatJID: String) -> [Message] {
        messages[chatJID] ?? []
    }

    func deleteChat(_ chat: Chat) {
        chats.removeAll { $0.jid == chat.jid }
        messages.removeValue(forKey: chat.jid)
    }

    func archiveChat(_ chat: Chat) {
        if let index = chats.firstIndex(where: { $0.jid == chat.jid }) {
            chats[index].isArchived = true
        }
    }

    func pinChat(_ chat: Chat) {
        if let index = chats.firstIndex(where: { $0.jid == chat.jid }) {
            chats[index].isPinned.toggle()
        }
    }

    func muteChat(_ chat: Chat) {
        if let index = chats.firstIndex(where: { $0.jid == chat.jid }) {
            chats[index].isMuted.toggle()
        }
    }

    // MARK: - Contact Management

    func refreshContacts() async {
        do {
            contacts = try await client.getStoredContacts()
        } catch {
            print("Failed to refresh contacts: \(error)")
        }
    }

    func startChat(with contact: Contact) -> Chat {
        // Check if chat already exists
        if let existingChat = chats.first(where: { $0.jid == contact.jid }) {
            return existingChat
        }

        // Create new chat
        let newChat = Chat(
            jid: contact.jid,
            name: contact.displayName,
            lastMessage: "",
            lastMessageTime: Date(),
            unreadCount: 0,
            isGroup: contact.isGroup,
            isMuted: false,
            isPinned: false,
            isArchived: false
        )

        chats.insert(newChat, at: 0)
        return newChat
    }

    func startChat(withPhoneNumber phoneNumber: String) async -> Chat? {
        let jid = WhatsAppClient.createJID(phoneNumber: phoneNumber)

        // Check if already exists
        if let existingChat = chats.first(where: { $0.jid == jid }) {
            return existingChat
        }

        // Check if on WhatsApp
        do {
            let isOnWhatsApp = try await client.isOnWhatsApp(phoneNumber: phoneNumber)
            if !isOnWhatsApp {
                connectionError = "This number is not on WhatsApp"
                return nil
            }
        } catch {
            connectionError = error.localizedDescription
            return nil
        }

        let newChat = Chat(
            jid: jid,
            name: phoneNumber,
            lastMessage: "",
            lastMessageTime: Date(),
            unreadCount: 0,
            isGroup: false,
            isMuted: false,
            isPinned: false,
            isArchived: false
        )

        chats.insert(newChat, at: 0)
        return newChat
    }

    // MARK: - Group Management

    func createGroup(name: String, participants: [Contact]) async -> Chat? {
        do {
            let participantJIDs = participants.map { $0.jid }
            let groupJID = try await client.createGroup(name: name, participants: participantJIDs)

            let newChat = Chat(
                jid: groupJID,
                name: name,
                lastMessage: "You created this group",
                lastMessageTime: Date(),
                unreadCount: 0,
                isGroup: true,
                isMuted: false,
                isPinned: false,
                isArchived: false,
                participantCount: participants.count + 1
            )

            chats.insert(newChat, at: 0)
            return newChat
        } catch {
            connectionError = error.localizedDescription
            return nil
        }
    }

    func leaveGroup(_ groupJID: String) async {
        do {
            try await client.leaveGroup(groupJID: groupJID)
            chats.removeAll { $0.jid == groupJID }
        } catch {
            connectionError = error.localizedDescription
        }
    }
}

// MARK: - WhatsAppClientDelegate

extension ChatViewModel: WhatsAppClientDelegate {
    nonisolated func didReceiveQRCode(_ code: String) {
        Task { @MainActor in
            self.currentQRCode = code
            self.isConnecting = false
        }
    }

    nonisolated func didConnect() {
        Task { @MainActor in
            self.isConnected = true
            self.isLoggedIn = true
            self.currentQRCode = nil
            self.isConnecting = false
            UserDefaults.standard.set(true, forKey: .isLoggedIn)
        }
    }

    nonisolated func didDisconnect(reason: String) {
        Task { @MainActor in
            self.isConnected = false
            if reason != "User disconnected" {
                self.connectionError = reason
            }
        }
    }

    nonisolated func didLogout(reason: String) {
        Task { @MainActor in
            self.isConnected = false
            self.isLoggedIn = false
            self.myJID = nil
            UserDefaults.standard.set(false, forKey: .isLoggedIn)
        }
    }

    nonisolated func didReceiveMessage(_ message: Message) {
        Task { @MainActor in
            // Add message to storage
            if self.messages[message.chatJID] == nil {
                self.messages[message.chatJID] = []
            }
            self.messages[message.chatJID]?.append(message)

            // Update chat
            if let index = self.chats.firstIndex(where: { $0.jid == message.chatJID }) {
                self.chats[index].lastMessage = message.displayText
                self.chats[index].lastMessageTime = message.timestamp
                if !message.isFromMe {
                    self.chats[index].unreadCount += 1
                }
            } else {
                // Create new chat
                let newChat = Chat(
                    jid: message.chatJID,
                    name: message.senderName,
                    lastMessage: message.displayText,
                    lastMessageTime: message.timestamp,
                    unreadCount: 1,
                    isGroup: message.isGroup,
                    isMuted: false,
                    isPinned: false,
                    isArchived: false
                )
                self.chats.insert(newChat, at: 0)
            }

            HapticFeedback.light()
        }
    }

    nonisolated func didReceiveReceipt(_ receipt: MessageReceipt) {
        Task { @MainActor in
            if let messages = self.messages[receipt.chatJID],
               let index = messages.firstIndex(where: { $0.id == receipt.messageID }) {
                switch receipt.type {
                case .delivered:
                    self.messages[receipt.chatJID]?[index].status = .delivered
                case .read:
                    self.messages[receipt.chatJID]?[index].status = .read
                default:
                    break
                }
            }
        }
    }

    nonisolated func didReceivePresence(_ presence: UserPresence) {
        // Update contact presence if needed
    }

    nonisolated func didReceiveHistorySync(progress: Int, total: Int) {
        // Handle history sync progress
    }

    nonisolated func didReceiveError(_ error: String) {
        Task { @MainActor in
            self.connectionError = error
        }
    }
}
