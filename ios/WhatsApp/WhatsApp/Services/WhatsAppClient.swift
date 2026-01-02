import Foundation
import Combine
import Mobile  // Import the Go mobile framework

/// Protocol for WhatsApp event callbacks
protocol WhatsAppClientDelegate: AnyObject {
    func didReceiveQRCode(_ code: String)
    func didConnect()
    func didDisconnect(reason: String)
    func didLogout(reason: String)
    func didReceiveMessage(_ message: Message)
    func didReceiveReceipt(_ receipt: MessageReceipt)
    func didReceivePresence(_ presence: UserPresence)
    func didReceiveHistorySync(progress: Int, total: Int)
    func didReceiveError(_ error: String)
}

/// Message receipt information
struct MessageReceipt {
    let messageID: String
    let chatJID: String
    let senderJID: String
    let type: ReceiptType
    let timestamp: Date

    enum ReceiptType: String {
        case delivered
        case read
        case played
        case unknown
    }
}

/// User presence information
struct UserPresence {
    let jid: String
    let isAvailable: Bool
    let lastSeen: Date?
}

/// WhatsApp client wrapper for iOS
/// This class wraps the Go mobile framework and provides a Swift-friendly API
class WhatsAppClient: NSObject, ObservableObject {
    static let shared = WhatsAppClient()

    // Published properties for SwiftUI bindings
    @Published private(set) var isConnected = false
    @Published private(set) var isLoggedIn = false
    @Published private(set) var currentQRCode: String?
    @Published private(set) var myJID: String?
    @Published private(set) var connectionError: String?

    weak var delegate: WhatsAppClientDelegate?

    // Go client from the Mobile framework
    private var goClient: MobileClient?
    private let queue = DispatchQueue(label: "com.whatsapp.client", qos: .userInitiated)

    private override init() {
        super.init()
    }

    // MARK: - Database Path

    private var databasePath: String {
        let documentsPath = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first!
        return documentsPath.appendingPathComponent("whatsapp.db").path
    }

    // MARK: - Connection Management

    /// Initialize the WhatsApp client
    func initialize() {
        queue.async { [weak self] in
            guard let self = self else { return }

            do {
                // Create the Go client with database path and self as callback
                var error: NSError?
                let client = MobileNewClient(self.databasePath, self, &error)

                if let error = error {
                    throw error
                }

                self.goClient = client
                print("WhatsApp client initialized with database at: \(self.databasePath)")
            } catch {
                DispatchQueue.main.async {
                    self.connectionError = "Failed to initialize: \(error.localizedDescription)"
                    self.delegate?.didReceiveError(error.localizedDescription)
                }
            }
        }
    }

    /// Connect to WhatsApp servers
    func connect() {
        queue.async { [weak self] in
            guard let self = self, let client = self.goClient else {
                DispatchQueue.main.async {
                    self?.connectionError = "Client not initialized"
                }
                return
            }

            do {
                try client.connect()
            } catch {
                DispatchQueue.main.async {
                    self.connectionError = "Connection failed: \(error.localizedDescription)"
                    self.delegate?.didReceiveError(error.localizedDescription)
                }
            }
        }
    }

    /// Disconnect from WhatsApp
    func disconnect() {
        queue.async { [weak self] in
            guard let self = self else { return }

            self.goClient?.disconnect()

            DispatchQueue.main.async {
                self.isConnected = false
                self.delegate?.didDisconnect(reason: "User disconnected")
            }
        }
    }

    /// Logout and unpair the device
    func logout() async throws {
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [weak self] in
                guard let self = self, let client = self.goClient else {
                    continuation.resume(throwing: WhatsAppError.clientNotInitialized)
                    return
                }

                do {
                    try client.logout()

                    DispatchQueue.main.async {
                        self.isLoggedIn = false
                        self.isConnected = false
                        self.myJID = nil
                        self.delegate?.didLogout(reason: "User logged out")
                    }

                    continuation.resume()
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    // MARK: - Messaging

    /// Send a text message
    func sendTextMessage(to chatJID: String, text: String) async throws -> String {
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [weak self] in
                guard let client = self?.goClient else {
                    continuation.resume(throwing: WhatsAppError.clientNotInitialized)
                    return
                }

                do {
                    var error: NSError?
                    let messageID = client.sendTextMessage(chatJID, text: text, error: &error)

                    if let error = error {
                        throw error
                    }

                    continuation.resume(returning: messageID)
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    /// Send an image message
    func sendImageMessage(to chatJID: String, imageData: Data, caption: String, mimeType: String = "image/jpeg") async throws -> String {
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [weak self] in
                guard let client = self?.goClient else {
                    continuation.resume(throwing: WhatsAppError.clientNotInitialized)
                    return
                }

                do {
                    let base64Data = imageData.base64EncodedString()
                    var error: NSError?
                    let messageID = client.sendImageMessage(chatJID, imageDataBase64: base64Data, caption: caption, mimeType: mimeType, error: &error)

                    if let error = error {
                        throw error
                    }

                    continuation.resume(returning: messageID)
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    /// Send a document
    func sendDocumentMessage(to chatJID: String, documentData: Data, filename: String, caption: String, mimeType: String) async throws -> String {
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [weak self] in
                guard let client = self?.goClient else {
                    continuation.resume(throwing: WhatsAppError.clientNotInitialized)
                    return
                }

                do {
                    let base64Data = documentData.base64EncodedString()
                    var error: NSError?
                    let messageID = client.sendDocumentMessage(chatJID, documentDataBase64: base64Data, filename: filename, caption: caption, mimeType: mimeType, error: &error)

                    if let error = error {
                        throw error
                    }

                    continuation.resume(returning: messageID)
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    /// Mark messages as read
    func markAsRead(chatJID: String, messageIDs: [String]) async throws {
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [weak self] in
                guard let client = self?.goClient else {
                    continuation.resume(throwing: WhatsAppError.clientNotInitialized)
                    return
                }

                do {
                    let idsJSON = try String(data: JSONEncoder().encode(messageIDs), encoding: .utf8)!
					try client.mark(asRead: chatJID, messageIDsJSON: idsJSON)
                    continuation.resume()
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    /// Send typing indicator
    func sendTyping(to chatJID: String, isTyping: Bool) async throws {
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [weak self] in
                guard let client = self?.goClient else {
                    continuation.resume(throwing: WhatsAppError.clientNotInitialized)
                    return
                }

                do {
                    try client.sendTyping(chatJID, typing: isTyping)
                    continuation.resume()
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    // MARK: - Contacts & Groups

    /// Get contact information
    func getContactInfo(jid: String) async throws -> Contact {
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [weak self] in
                guard let client = self?.goClient else {
                    continuation.resume(throwing: WhatsAppError.clientNotInitialized)
                    return
                }

                do {
                    var error: NSError?
                    let mobileContact = try client.getContactInfo(jid)

                    if let error = error {
                        throw error
                    }

                    let contact = Contact(
                        jid: mobileContact.jid,
                        name: mobileContact.name,
                        pushName: mobileContact.pushName,
                        phoneNumber: mobileContact.phoneNumber,
                        isGroup: mobileContact.isGroup
                    )

                    continuation.resume(returning: contact)
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    /// Get all stored contacts
    func getStoredContacts() async throws -> [Contact] {
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [weak self] in
                guard let client = self?.goClient else {
                    continuation.resume(throwing: WhatsAppError.clientNotInitialized)
                    return
                }

                do {
                    var error: NSError?
                    let contactsJSON = client.getStoredContacts(&error)

                    if let error = error {
                        throw error
                    }

                    guard let jsonData = contactsJSON.data(using: .utf8) else {
                        throw WhatsAppError.unknown("Invalid JSON response")
                    }

                    let contacts = try JSONDecoder().decode([Contact].self, from: jsonData)
                    continuation.resume(returning: contacts)
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    /// Get all joined groups
    func getJoinedGroups() async throws -> [GroupInfoResponse] {
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [weak self] in
                guard let client = self?.goClient else {
                    continuation.resume(throwing: WhatsAppError.clientNotInitialized)
                    return
                }

                do {
                    var error: NSError?
                    let groupsJSON = client.getJoinedGroups(&error)

                    if let error = error {
                        throw error
                    }

                    guard let jsonData = groupsJSON.data(using: .utf8) else {
                        throw WhatsAppError.unknown("Invalid JSON response")
                    }

                    let groups = try JSONDecoder().decode([GroupInfoResponse].self, from: jsonData)
                    continuation.resume(returning: groups)
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

   
	/// Check if a phone number is on WhatsApp
    func isOnWhatsApp(phoneNumber: String) async throws -> Bool {
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [weak self] in
                guard let client = self?.goClient else {
                    continuation.resume(throwing: WhatsAppError.clientNotInitialized)
                    return
                }

                do {
					var result: ObjCBool = false as ObjCBool
					try client.is(onWhatsApp: phoneNumber, ret0_: &result)

					continuation.resume(returning: result.boolValue)
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    /// Get profile picture URL
    func getProfilePicture(jid: String) async throws -> String? {
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [weak self] in
                guard let client = self?.goClient else {
                    continuation.resume(throwing: WhatsAppError.clientNotInitialized)
                    return
                }

                do {
                    var error: NSError?
                    let url = client.getProfilePicture(jid, error: &error)

                    if let error = error {
                        throw error
                    }

                    continuation.resume(returning: url.isEmpty ? nil : url)
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    /// Create a new group
    func createGroup(name: String, participants: [String]) async throws -> String {
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [weak self] in
                guard let client = self?.goClient else {
                    continuation.resume(throwing: WhatsAppError.clientNotInitialized)
                    return
                }

                do {
                    let participantsJSON = try String(data: JSONEncoder().encode(participants), encoding: .utf8)!
                    var error: NSError?
                    let groupJID = client.createGroup(name, participantsJSON: participantsJSON, error: &error)

                    if let error = error {
                        throw error
                    }

                    continuation.resume(returning: groupJID)
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    /// Leave a group
    func leaveGroup(groupJID: String) async throws {
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [weak self] in
                guard let client = self?.goClient else {
                    continuation.resume(throwing: WhatsAppError.clientNotInitialized)
                    return
                }

                do {
                    try client.leaveGroup(groupJID)
                    continuation.resume()
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    // MARK: - Presence

    /// Set online/offline presence
    func setPresence(available: Bool) async throws {
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [weak self] in
                guard let client = self?.goClient else {
                    continuation.resume(throwing: WhatsAppError.clientNotInitialized)
                    return
                }

                do {
                    try client.setPresence(available)
                    continuation.resume()
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    // MARK: - Utility

    /// Create a JID from a phone number
    static func createJID(phoneNumber: String) -> String {
        // Use the Go utility function
        return MobileCreateJID(phoneNumber)
    }

    /// Create a group JID
    static func createGroupJID(groupID: String) -> String {
        return MobileCreateGroupJID(groupID)
    }

    /// Get the current user's JID
    func getMyJIDString() -> String? {
        return goClient?.getMyJID()
    }

    /// Check if connected
    func checkIsConnected() -> Bool {
        return goClient?.isConnected() ?? false
    }

    /// Check if logged in
    func checkIsLoggedIn() -> Bool {
        return goClient?.isLoggedIn() ?? false
    }
}

// MARK: - MobileEventCallbackProtocol Implementation

extension WhatsAppClient: MobileEventCallbackProtocol {

    func onQRCode(_ code: String?) {
        guard let code = code, !code.isEmpty else { return }
        DispatchQueue.main.async {
            self.currentQRCode = code
            self.delegate?.didReceiveQRCode(code)
        }
    }

    func onConnected() {
        DispatchQueue.main.async {
            self.isConnected = true
            self.isLoggedIn = true
            self.currentQRCode = nil
            self.myJID = self.goClient?.getMyJID()
            self.delegate?.didConnect()
        }
    }

    func onDisconnected(_ reason: String?) {
        DispatchQueue.main.async {
            self.isConnected = false
            self.delegate?.didDisconnect(reason: reason ?? "Unknown")
        }
    }

    func onLoggedOut(_ reason: String?) {
        DispatchQueue.main.async {
            self.isConnected = false
            self.isLoggedIn = false
            self.myJID = nil
            self.delegate?.didLogout(reason: reason ?? "Unknown")
        }
    }

    func onMessage(_ msg: MobileMessage?) {
        guard let msg = msg else { return }

        let mediaType: Message.MediaType
        switch msg.mediaType {
        case "image": mediaType = .image
        case "video": mediaType = .video
        case "audio": mediaType = .audio
        case "document": mediaType = .document
        case "sticker": mediaType = .sticker
        default: mediaType = .none
        }

        let message = Message(
			id: msg.id_,
            chatJID: msg.chatJID,
            senderJID: msg.senderJID,
            senderName: msg.senderName,
            text: msg.text,
            timestamp: Date(timeIntervalSince1970: TimeInterval(msg.timestamp)),
            isFromMe: msg.isFromMe,
            isGroup: msg.isGroup,
            mediaType: mediaType,
            mediaURL: msg.mediaURL.isEmpty ? nil : msg.mediaURL,
            mediaCaption: msg.mediaCaption.isEmpty ? nil : msg.mediaCaption,
            quotedID: msg.quotedID.isEmpty ? nil : msg.quotedID,
            quotedText: msg.quotedText.isEmpty ? nil : msg.quotedText,
            status: .delivered
        )

        DispatchQueue.main.async {
            self.delegate?.didReceiveMessage(message)
        }
    }

    func onReceipt(_ receipt: MobileReceipt?) {
        guard let receipt = receipt else { return }

        let receiptType: MessageReceipt.ReceiptType
        switch receipt.type {
        case "delivered": receiptType = .delivered
        case "read": receiptType = .read
        case "played": receiptType = .played
        default: receiptType = .unknown
        }

        let messageReceipt = MessageReceipt(
            messageID: receipt.messageID,
            chatJID: receipt.chatJID,
            senderJID: receipt.senderJID,
            type: receiptType,
            timestamp: Date(timeIntervalSince1970: TimeInterval(receipt.timestamp))
        )

        DispatchQueue.main.async {
            self.delegate?.didReceiveReceipt(messageReceipt)
        }
    }

    func onPresence(_ presence: MobilePresence?) {
        guard let presence = presence else { return }

        let userPresence = UserPresence(
            jid: presence.jid,
            isAvailable: presence.available,
            lastSeen: presence.lastSeen > 0 ? Date(timeIntervalSince1970: TimeInterval(presence.lastSeen)) : nil
        )

        DispatchQueue.main.async {
            self.delegate?.didReceivePresence(userPresence)
        }
    }

    func onHistorySync(_ progress: Int, total: Int) {
        DispatchQueue.main.async {
            self.delegate?.didReceiveHistorySync(progress: progress, total: total)
        }
    }

    func onError(_ err: String?) {
        guard let err = err, !err.isEmpty else { return }
        DispatchQueue.main.async {
            self.connectionError = err
            self.delegate?.didReceiveError(err)
        }
    }
}

// MARK: - Group Info Response (for JSON decoding)

struct GroupInfoResponse: Codable {
    let jid: String
    let name: String
    let topic: String
    let participantCount: Int
    let createdAt: Int64
    let isAdmin: Bool

    enum CodingKeys: String, CodingKey {
        case jid = "JID"
        case name = "Name"
        case topic = "Topic"
        case participantCount = "ParticipantCount"
        case createdAt = "CreatedAt"
        case isAdmin = "IsAdmin"
    }
}

// MARK: - Errors

enum WhatsAppError: LocalizedError {
    case clientNotInitialized
    case notConnected
    case notLoggedIn
    case invalidJID
    case sendFailed(String)
    case unknown(String)

    var errorDescription: String? {
        switch self {
        case .clientNotInitialized:
            return "WhatsApp client not initialized"
        case .notConnected:
            return "Not connected to WhatsApp"
        case .notLoggedIn:
            return "Not logged in"
        case .invalidJID:
            return "Invalid JID format"
        case .sendFailed(let reason):
            return "Failed to send message: \(reason)"
        case .unknown(let message):
            return message
        }
    }
}
