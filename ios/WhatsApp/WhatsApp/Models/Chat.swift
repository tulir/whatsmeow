import Foundation

/// Represents a WhatsApp chat/conversation
struct Chat: Identifiable, Codable, Hashable {
    let jid: String
    var name: String
    var lastMessage: String
    var lastMessageTime: Date
    var unreadCount: Int
    let isGroup: Bool
    var isMuted: Bool
    var isPinned: Bool
    var isArchived: Bool
    var profilePictureURL: String?
    var participantCount: Int?

    var id: String { jid }

    var displayName: String {
        name.isEmpty ? jid.components(separatedBy: "@").first ?? jid : name
    }

    var lastMessageTimeFormatted: String {
        let calendar = Calendar.current

        if calendar.isDateInToday(lastMessageTime) {
            let formatter = DateFormatter()
            formatter.dateFormat = "HH:mm"
            return formatter.string(from: lastMessageTime)
        } else if calendar.isDateInYesterday(lastMessageTime) {
            return "Yesterday"
        } else if let daysAgo = calendar.dateComponents([.day], from: lastMessageTime, to: Date()).day,
                  daysAgo < 7 {
            let formatter = DateFormatter()
            formatter.dateFormat = "EEEE"
            return formatter.string(from: lastMessageTime)
        } else {
            let formatter = DateFormatter()
            formatter.dateFormat = "dd/MM/yy"
            return formatter.string(from: lastMessageTime)
        }
    }

    var initials: String {
        let words = displayName.split(separator: " ")
        if words.count >= 2 {
            return String(words[0].prefix(1) + words[1].prefix(1)).uppercased()
        }
        return String(displayName.prefix(2)).uppercased()
    }

    static func == (lhs: Chat, rhs: Chat) -> Bool {
        lhs.jid == rhs.jid
    }

    func hash(into hasher: inout Hasher) {
        hasher.combine(jid)
    }
}

// MARK: - Sample Data
extension Chat {
    static let sample = Chat(
        jid: "1234567890@s.whatsapp.net",
        name: "John Doe",
        lastMessage: "Hey, how are you?",
        lastMessageTime: Date(),
        unreadCount: 2,
        isGroup: false,
        isMuted: false,
        isPinned: true,
        isArchived: false
    )

    static let samples: [Chat] = [
        Chat(
            jid: "1111111111@s.whatsapp.net",
            name: "Alice Johnson",
            lastMessage: "See you tomorrow!",
            lastMessageTime: Date().addingTimeInterval(-300),
            unreadCount: 1,
            isGroup: false,
            isMuted: false,
            isPinned: true,
            isArchived: false
        ),
        Chat(
            jid: "2222222222@g.us",
            name: "Family Group",
            lastMessage: "Mom: Don't forget the dinner!",
            lastMessageTime: Date().addingTimeInterval(-3600),
            unreadCount: 5,
            isGroup: true,
            isMuted: false,
            isPinned: true,
            isArchived: false,
            participantCount: 8
        ),
        Chat(
            jid: "3333333333@s.whatsapp.net",
            name: "Bob Smith",
            lastMessage: "Thanks for the help!",
            lastMessageTime: Date().addingTimeInterval(-7200),
            unreadCount: 0,
            isGroup: false,
            isMuted: true,
            isPinned: false,
            isArchived: false
        ),
        Chat(
            jid: "4444444444@g.us",
            name: "Work Team",
            lastMessage: "Meeting at 3pm",
            lastMessageTime: Date().addingTimeInterval(-86400),
            unreadCount: 10,
            isGroup: true,
            isMuted: false,
            isPinned: false,
            isArchived: false,
            participantCount: 15
        ),
    ]
}
