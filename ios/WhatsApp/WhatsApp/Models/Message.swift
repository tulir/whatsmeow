import Foundation

/// Represents a WhatsApp message
struct Message: Identifiable, Codable, Hashable {
    let id: String
    let chatJID: String
    let senderJID: String
    var senderName: String
    var text: String
    let timestamp: Date
    let isFromMe: Bool
    let isGroup: Bool
    var mediaType: MediaType
    var mediaURL: String?
    var mediaCaption: String?
    var quotedID: String?
    var quotedText: String?
    var status: MessageStatus

    enum MediaType: String, Codable {
        case none = ""
        case image
        case video
        case audio
        case document
        case sticker
    }

    enum MessageStatus: String, Codable {
        case sending
        case sent
        case delivered
        case read
        case failed
    }

    init(
        id: String,
        chatJID: String,
        senderJID: String,
        senderName: String = "",
        text: String,
        timestamp: Date = Date(),
        isFromMe: Bool = false,
        isGroup: Bool = false,
        mediaType: MediaType = .none,
        mediaURL: String? = nil,
        mediaCaption: String? = nil,
        quotedID: String? = nil,
        quotedText: String? = nil,
        status: MessageStatus = .sending
    ) {
        self.id = id
        self.chatJID = chatJID
        self.senderJID = senderJID
        self.senderName = senderName
        self.text = text
        self.timestamp = timestamp
        self.isFromMe = isFromMe
        self.isGroup = isGroup
        self.mediaType = mediaType
        self.mediaURL = mediaURL
        self.mediaCaption = mediaCaption
        self.quotedID = quotedID
        self.quotedText = quotedText
        self.status = status
    }

    var displayText: String {
        if !text.isEmpty {
            return text
        }
        if let caption = mediaCaption, !caption.isEmpty {
            return caption
        }
        switch mediaType {
        case .image: return "Photo"
        case .video: return "Video"
        case .audio: return "Audio"
        case .document: return "Document"
        case .sticker: return "Sticker"
        case .none: return ""
        }
    }

    var hasMedia: Bool {
        mediaType != .none
    }
}

// MARK: - Sample Data
extension Message {
    static let sample = Message(
        id: "sample-1",
        chatJID: "1234567890@s.whatsapp.net",
        senderJID: "1234567890@s.whatsapp.net",
        senderName: "John Doe",
        text: "Hello! This is a sample message.",
        timestamp: Date(),
        isFromMe: false,
        isGroup: false
    )

    static let sampleOutgoing = Message(
        id: "sample-2",
        chatJID: "1234567890@s.whatsapp.net",
        senderJID: "me@s.whatsapp.net",
        senderName: "Me",
        text: "Hi there! How are you?",
        timestamp: Date(),
        isFromMe: true,
        isGroup: false,
        status: .read
    )
}
