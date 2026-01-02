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
    @Published var chats: [Chat] = [] {
        didSet {
            invalidateCache()
        }
    }
    @Published var contacts: [Contact] = []
    @Published var messages: [String: [Message]] = [:] // chatJID -> messages
    @Published var selectedChat: Chat?

    // UI state
    @Published var isLoadingChats = false
    @Published var isLoadingMessages = false
    @Published var searchText = "" // Cache invalidation is debounced in setupBindings()

    // Sync state
    @Published var isSyncing = false
    @Published var syncProgress: Int = 0
    @Published var syncTotal: Int = 0

    // MARK: - Private Properties

    private let client = WhatsAppClient.shared
    private var cancellables = Set<AnyCancellable>()
    private let persistence = PersistenceController.shared

    // Cache for computed properties
    private var _cachedSortedChats: [Chat]?
    private var _cachedFilteredChats: [Chat]?
    private let cacheQueue = DispatchQueue(label: "com.whatsapp.cache", qos: .userInitiated)

    // Batch update mechanism
    private var pendingUIUpdates: [() -> Void] = []
    private var updateWorkItem: DispatchWorkItem?
    private let updateQueue = DispatchQueue(label: "com.whatsapp.updates", qos: .userInitiated)

    // Profile picture loading queue (serial to prevent concurrent Go calls)
    private let profileQueue = DispatchQueue(label: "com.whatsapp.profile", qos: .utility)
    private var loadingProfileJIDs = Set<String>()

    // MARK: - Computed Properties

    var filteredChats: [Chat] {
        if let cached = _cachedFilteredChats {
            return cached
        }

        let filtered: [Chat]
        if searchText.isEmpty {
            filtered = sortedChats
        } else {
            let lowerSearch = searchText.lowercased()
            filtered = sortedChats.filter {
                $0.name.lowercased().contains(lowerSearch) ||
                $0.lastMessage.lowercased().contains(lowerSearch)
            }
        }

        _cachedFilteredChats = filtered
        return filtered
    }

    var sortedChats: [Chat] {
        if let cached = _cachedSortedChats {
            return cached
        }

        let sorted = chats.sorted { chat1, chat2 in
            // Pinned chats first
            if chat1.isPinned != chat2.isPinned {
                return chat1.isPinned
            }
            // Then by last message time
            return chat1.lastMessageTime > chat2.lastMessageTime
        }

        _cachedSortedChats = sorted
        return sorted
    }

    private func invalidateCache() {
        _cachedSortedChats = nil
        _cachedFilteredChats = nil
    }

    private func invalidateFilterCache() {
        _cachedFilteredChats = nil
    }

    // MARK: - Batch UI Updates

    /// Schedule a UI update to be batched (reduces main thread load)
    private func scheduleBatchUpdate(_ update: @escaping () -> Void) {
        updateQueue.async { [weak self] in
            guard let self = self else { return }
            self.pendingUIUpdates.append(update)

            // Cancel previous scheduled flush
            self.updateWorkItem?.cancel()

            // Schedule flush after 100ms of inactivity
            let workItem = DispatchWorkItem { [weak self] in
                self?.flushPendingUpdates()
            }
            self.updateWorkItem = workItem

            DispatchQueue.main.asyncAfter(deadline: .now() + 0.1, execute: workItem)
        }
    }

    /// Execute all pending UI updates at once
    private func flushPendingUpdates() {
        updateQueue.async { [weak self] in
            guard let self = self else { return }
            let updates = self.pendingUIUpdates
            self.pendingUIUpdates = []

            // Execute all updates on main thread in a single batch
            DispatchQueue.main.async {
                updates.forEach { $0() }
            }
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
        // Load persisted data asynchronously on background thread
        Task {
            await loadPersistedData()
        }
    }

    // MARK: - Persistence (CoreData)

    private func loadPersistedData() async {
        // Load from CoreData on background thread
        let loadedChats = await persistence.fetchAllChatsAsync()
        let loadedMessages = await persistence.fetchRecentMessagesAsync(limit: 50) // Load only recent messages

        // Update UI on main thread
        await MainActor.run {
            self.chats = loadedChats
            self.messages = loadedMessages
        }
    }

    private func persistMessage(_ message: Message) {
        // Queue for batch save (debounced)
        persistence.queueMessage(message)
    }

    private func persistChat(_ chat: Chat) {
        // Queue for batch save (debounced)
        persistence.queueChatUpdate(chat)
    }

    private func addMessage(_ message: Message) {
        // Add to in-memory cache, avoiding duplicates
        if messages[message.chatJID] == nil {
            messages[message.chatJID] = []
        }

        // Check for duplicate
        guard messages[message.chatJID]?.contains(where: { $0.id == message.id }) == false else {
            return
        }

        // Insert in sorted position (binary search for better performance)
        let chatMessages = messages[message.chatJID]!
        let insertIndex = chatMessages.insertionIndex(of: message) { $0.timestamp < $1.timestamp }
        messages[message.chatJID]?.insert(message, at: insertIndex)

        // Persist to CoreData (batched, on background thread)
        persistMessage(message)
    }

    private func updateOrCreateChat(for message: Message) {
        if let index = chats.firstIndex(where: { $0.jid == message.chatJID }) {
            // Update existing chat if message is newer
            if message.timestamp > chats[index].lastMessageTime {
                chats[index].lastMessage = message.displayText
                chats[index].lastMessageTime = message.timestamp
            }
            if !message.isFromMe {
                chats[index].unreadCount += 1
            }
            // Persist updated chat
            persistChat(chats[index])
        } else {
            // Create new chat
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
            chats.append(newChat)
            // Persist new chat
            persistChat(newChat)

            // Profile picture will be loaded when chat row appears
        }
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

        // Debounce search text updates to reduce UI recalculations
        $searchText
            .debounce(for: .milliseconds(300), scheduler: DispatchQueue.main)
            .sink { [weak self] _ in
                // Trigger cache invalidation only after user stops typing
                self?.invalidateFilterCache()
            }
            .store(in: &cancellables)
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
            // Load contacts (avoiding database during sync reduces conflicts)
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
                    // Persist chat to CoreData (now safe after sync completes)
                    persistChat(chat)
                }
            }

            print("Loaded \(contacts.count) contacts and \(groups.count) groups")

        } catch {
            print("Failed to load initial data: \(error)")
        }

        isLoadingChats = false
    }

    /// Load profile pictures for all chats
    private func loadAllProfilePictures() async {
        for chat in chats {
            // Skip if already has profile picture
            if chat.profilePictureURL != nil && !chat.profilePictureURL!.isEmpty {
                continue
            }

            await loadProfilePicture(for: chat.jid)

            // Rate limit to avoid overwhelming the server
            try? await Task.sleep(nanoseconds: 100_000_000) // 100ms between requests
        }
    }

    /// Request profile picture loading for a JID (DISABLED - causes CGO crashes)
    func requestProfilePicture(for jid: String) {
        // TEMPORARILY DISABLED: GetProfilePicture() causes fatal CGO crashes
        // Error: bulkBarrierPreWrite: unaligned arguments
        // TODO: Fix Go Mobile framework integration before re-enabling
        return
    }

    /// Load profile picture for a specific JID (DISABLED)
    private func loadProfilePicture(for jid: String) async {
        // DISABLED: CGO crashes with Go Mobile framework
        return
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
                let updatedMessage = Message(
                    id: messageID,
                    chatJID: chatJID,
                    senderJID: myJID ?? "",
                    senderName: "Me",
                    text: text,
                    timestamp: localMessage.timestamp,
                    isFromMe: true,
                    isGroup: chatJID.isGroupJID,
                    status: .sent
                )
                messages[chatJID]?[msgIndex] = updatedMessage

                // Persist to CoreData
                persistMessage(updatedMessage)
                if let index = chats.firstIndex(where: { $0.jid == chatJID }) {
                    persistChat(chats[index])
                }
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
        // Update unread count immediately for better UX
        if let index = chats.firstIndex(where: { $0.jid == chatJID }) {
            chats[index].unreadCount = 0
            // Persist chat update
            persistChat(chats[index])
        }

        guard let chatMessages = messages[chatJID] else { return }

        let unreadMessageIDs = chatMessages
            .filter { !$0.isFromMe && $0.status != .read }
            .map { $0.id }

        guard !unreadMessageIDs.isEmpty else { return }

        do {
            try await client.markAsRead(chatJID: chatJID, messageIDs: unreadMessageIDs)

            // Update message statuses to read
            for (idx, message) in chatMessages.enumerated() where unreadMessageIDs.contains(message.id) {
                messages[chatJID]?[idx].status = .read
                persistMessage(messages[chatJID]![idx])
            }
        } catch {
            print("Failed to mark as read: \(error)")
            // Revert unread count on failure
            if let index = chats.firstIndex(where: { $0.jid == chatJID }) {
                chats[index].unreadCount = unreadMessageIDs.count
                persistChat(chats[index])
            }
        }
    }

    func sendTyping(to chatJID: String, isTyping: Bool) {
        Task {
            try? await client.sendTyping(to: chatJID, isTyping: isTyping)
        }
    }

    // MARK: - Chat Management

    /// Load messages for a specific chat with pagination
    func loadMessages(for chatJID: String, loadMore: Bool = false) async {
        isLoadingMessages = true

        let currentCount = messages[chatJID]?.count ?? 0
        let offset = loadMore ? currentCount : 0
        let limit = 50

        // Fetch from CoreData on background thread
        let loadedMessages = await persistence.fetchMessagesAsync(for: chatJID, limit: limit, offset: offset)

        // Update UI on main thread
        await MainActor.run {
            if loadMore {
                // Append older messages
                if messages[chatJID] == nil {
                    messages[chatJID] = loadedMessages
                } else {
                    messages[chatJID]?.insert(contentsOf: loadedMessages, at: 0)
                }
            } else {
                // Replace with fresh load
                messages[chatJID] = loadedMessages
            }
            isLoadingMessages = false
        }
    }

    func getMessages(for chatJID: String) -> [Message] {
        messages[chatJID] ?? []
    }

    /// Load more messages when scrolling to top
    func loadMoreMessages(for chatJID: String) async {
        await loadMessages(for: chatJID, loadMore: true)
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
        persistChat(newChat)

        // Profile picture will be loaded when chat row appears

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

            // Set syncing state - wait for history sync to complete before loading data
            self.isSyncing = true

            // Extract phone number from JID
            if let jid = self.myJID {
                let phone = jid.components(separatedBy: "@").first ?? ""
                self.myPhoneNumber = "+\(phone)"
            }

            // Fallback: If sync doesn't complete within 10 seconds, load data anyway
            Task {
                try? await Task.sleep(nanoseconds: 10_000_000_000) // 10 seconds
                if self.isSyncing {
                    print("History sync timeout - loading data anyway")
                    self.isSyncing = false
                    await self.loadInitialData()
                }
            }
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
            // Clear CoreData
            self.persistence.clearAllData()
        }
    }

    nonisolated func didReceiveMessage(_ message: Message) {
        Task { @MainActor in
            // During sync, only update in-memory state, skip database writes
            // This prevents "database is locked" errors during WhatsApp app state sync
            if self.isSyncing {
                // Store in memory only during sync
                if self.messages[message.chatJID] == nil {
                    self.messages[message.chatJID] = []
                }

                // Check for duplicate
                guard self.messages[message.chatJID]?.contains(where: { $0.id == message.id }) == false else {
                    return
                }

                // Add to memory (no persistence during sync)
                let chatMessages = self.messages[message.chatJID]!
                let insertIndex = chatMessages.insertionIndex(of: message) { $0.timestamp < $1.timestamp }
                self.messages[message.chatJID]?.insert(message, at: insertIndex)

                return
            }

            // After sync completes, normal flow with persistence
            self.addMessage(message)
            self.updateOrCreateChat(for: message)

            HapticFeedback.light()
        }
    }

    nonisolated func didReceiveReceipt(_ receipt: MessageReceipt) {
        // Batch receipt updates to avoid excessive UI refreshes
        Task { @MainActor in
            self.scheduleBatchUpdate { [weak self] in
                guard let self = self else { return }
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
    }

    nonisolated func didReceivePresence(_ presence: UserPresence) {
        // Update contact presence if needed
        // Could be used to show "online" status
    }

    nonisolated func didReceiveHistorySync(progress: Int, total: Int) {
        Task { @MainActor in
            self.syncProgress = progress
            self.syncTotal = total
            print("History sync: \(progress)/\(total)")

            // When sync completes, load initial data
            if progress >= total && total > 0 {
                print("History sync completed - loading initial data")
                self.isSyncing = false

                // Small delay to let database finish any pending writes
                try? await Task.sleep(nanoseconds: 500_000_000) // 500ms

                // Now safe to load data without database conflicts
                await self.loadInitialData()
            }
        }
    }

    nonisolated func didReceiveError(_ error: String) {
        Task { @MainActor in
            self.connectionError = error
            self.isConnecting = false
        }
    }
}
