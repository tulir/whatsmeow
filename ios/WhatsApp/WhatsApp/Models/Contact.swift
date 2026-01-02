import Foundation

/// Represents a WhatsApp contact
struct Contact: Identifiable, Codable, Hashable {
    let jid: String
    var name: String
    var pushName: String
    var phoneNumber: String
    let isGroup: Bool
    var profilePictureURL: String?

    var id: String { jid }

    var displayName: String {
        if !name.isEmpty {
            return name
        }
        if !pushName.isEmpty {
            return pushName
        }
        return formattedPhoneNumber
    }

    var formattedPhoneNumber: String {
        // Basic phone number formatting
        let cleaned = phoneNumber.filter { $0.isNumber }
        if cleaned.count == 10 {
            let areaCode = String(cleaned.prefix(3))
            let middle = String(cleaned.dropFirst(3).prefix(3))
            let last = String(cleaned.suffix(4))
            return "(\(areaCode)) \(middle)-\(last)"
        }
        return "+" + cleaned
    }

    var initials: String {
        let words = displayName.split(separator: " ")
        if words.count >= 2 {
            return String(words[0].prefix(1) + words[1].prefix(1)).uppercased()
        }
        return String(displayName.prefix(2)).uppercased()
    }
}

// MARK: - Sample Data
extension Contact {
    static let sample = Contact(
        jid: "1234567890@s.whatsapp.net",
        name: "John Doe",
        pushName: "John",
        phoneNumber: "1234567890",
        isGroup: false
    )

    static let sampleGroup = Contact(
        jid: "123456789@g.us",
        name: "Family Group",
        pushName: "",
        phoneNumber: "",
        isGroup: true
    )

    static let samples: [Contact] = [
        Contact(jid: "1111111111@s.whatsapp.net", name: "Alice Johnson", pushName: "Alice", phoneNumber: "1111111111", isGroup: false),
        Contact(jid: "2222222222@s.whatsapp.net", name: "Bob Smith", pushName: "Bob", phoneNumber: "2222222222", isGroup: false),
        Contact(jid: "3333333333@s.whatsapp.net", name: "Carol Williams", pushName: "Carol", phoneNumber: "3333333333", isGroup: false),
        Contact(jid: "4444444444@g.us", name: "Work Team", pushName: "", phoneNumber: "", isGroup: true),
    ]
}
