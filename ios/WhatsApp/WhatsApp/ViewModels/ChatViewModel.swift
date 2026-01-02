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
	@Published var connectionError: String? {
		didSet {
			print(connectionError)
		}
	}

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
        // Set self as delegate
        client.delegate = self
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

        // Check if we have stored credentials - auto connect if logged in before
        if UserDefaults.standard.bool(forKey: .isLoggedIn) {
            connect()
        }
    }

    func connect() {
        isConnecting = true
        connectionError = nil
        client.connect()
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

    // MARK: - Data Loading

    /// Load contacts and groups after connection
    func loadInitialData() async {
        isLoadingChats = true

        do {
            // Load contacts
            contacts = try await client.getStoredContacts()

            // Load joined groups
            let groups = try await client.getJoinedGroups()

            // Convert groups to chats
            for group in groups {
                let chat = Chat(
                    jid: group.jid,
                    name: group.name,
                    lastMessage: "",
                    lastMessageTime: Date(timeIntervalSince1970: TimeInterval(group.createdAt)),
                    unreadCount: 0,
                    isGroup: true,
                    isMuted: false,
                    isPinned: false,
                    isArchived: false,
                    participantCount: group.participantCount
                )

                if !chats.contains(where: { $0.jid == chat.jid }) {
                    chats.append(chat)
                }
            }

            // Extract phone number from JID
            if let jid = myJID {
                let phone = jid.components(separatedBy: "@").first ?? ""
                myPhoneNumber = "+\(phone)"
            }

        } catch {
            print("Failed to load initial data: \(error)")
        }

        isLoadingChats = false
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

            // Update message status and ID
            if let msgIndex = messages[chatJID]?.firstIndex(where: { $0.id == localMessage.id }) {
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
            connectionError = error.localizedDescription
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
        // Messages are stored locally and received via callback
        // In a full implementation, you might fetch from a local database
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
            self.myJID = self.client.getMyJIDString()
            UserDefaults.standard.set(true, forKey: .isLoggedIn)

            // Load initial data after connection
            await self.loadInitialData()
        }
    }

    nonisolated func didDisconnect(reason: String) {
        Task { @MainActor in
            self.isConnected = false
            self.isConnecting = false
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
            self.chats = []
            self.contacts = []
            self.messages = [:]
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
                // Create new chat for incoming message
                let newChat = Chat(
                    jid: message.chatJID,
                    name: message.senderName.isEmpty ? message.senderJID : message.senderName,
                    lastMessage: message.displayText,
                    lastMessageTime: message.timestamp,
                    unreadCount: message.isFromMe ? 0 : 1,
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
        // Could be used to show "online" status
    }

    nonisolated func didReceiveHistorySync(progress: Int, total: Int) {
        // Handle history sync progress
        // Could show a progress indicator
        Task { @MainActor in
            print("History sync: \(progress)/\(total)")
        }
    }

    nonisolated func didReceiveError(_ error: String) {
        Task { @MainActor in
            self.connectionError = error
            self.isConnecting = false
        }
    }
}
