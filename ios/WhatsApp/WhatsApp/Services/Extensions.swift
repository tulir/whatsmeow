import Foundation
import SwiftUI

// MARK: - Date Extensions

extension Date {
    var messageTimeString: String {
        let formatter = DateFormatter()
        formatter.dateFormat = "HH:mm"
        return formatter.string(from: self)
    }

    var chatListTimeString: String {
        let calendar = Calendar.current

        if calendar.isDateInToday(self) {
            let formatter = DateFormatter()
            formatter.dateFormat = "HH:mm"
            return formatter.string(from: self)
        } else if calendar.isDateInYesterday(self) {
            return "Yesterday"
        } else if let daysAgo = calendar.dateComponents([.day], from: self, to: Date()).day,
                  daysAgo < 7 {
            let formatter = DateFormatter()
            formatter.dateFormat = "EEEE"
            return formatter.string(from: self)
        } else {
            let formatter = DateFormatter()
            formatter.dateFormat = "dd/MM/yy"
            return formatter.string(from: self)
        }
    }

    var fullDateTimeString: String {
        let formatter = DateFormatter()
        formatter.dateStyle = .medium
        formatter.timeStyle = .short
        return formatter.string(from: self)
    }
}

// MARK: - String Extensions

extension String {
    var isValidPhoneNumber: Bool {
        let phoneRegex = "^[+]?[0-9]{10,15}$"
        let phonePredicate = NSPredicate(format: "SELF MATCHES %@", phoneRegex)
        return phonePredicate.evaluate(with: self.filter { $0.isNumber || $0 == "+" })
    }

    var cleanedPhoneNumber: String {
        filter { $0.isNumber || $0 == "+" }
    }

    var isJID: Bool {
        contains("@s.whatsapp.net") || contains("@g.us")
    }

    var isGroupJID: Bool {
        contains("@g.us")
    }

    func toJID() -> String {
        if isJID {
            return self
        }
        return WhatsAppClient.createJID(phoneNumber: self)
    }
}

// MARK: - Color Extensions

extension Color {
    static let whatsappGreen = Color(red: 37/255, green: 211/255, blue: 102/255)
    static let whatsappDarkGreen = Color(red: 18/255, green: 140/255, blue: 126/255)
    static let whatsappLightGreen = Color(red: 220/255, green: 248/255, blue: 198/255)
    static let whatsappBlue = Color(red: 52/255, green: 183/255, blue: 241/255)
    static let chatBackground = Color(red: 236/255, green: 229/255, blue: 221/255)
    static let messageBubbleOut = Color(red: 217/255, green: 253/255, blue: 211/255)
    static let messageBubbleIn = Color.white
}

// MARK: - View Extensions

extension View {
    func hideKeyboard() {
        UIApplication.shared.sendAction(#selector(UIResponder.resignFirstResponder), to: nil, from: nil, for: nil)
    }

    @ViewBuilder
    func `if`<Content: View>(_ condition: Bool, transform: (Self) -> Content) -> some View {
        if condition {
            transform(self)
        } else {
            self
        }
    }
}

// MARK: - Data Extensions

extension Data {
    var prettyPrintedJSONString: String? {
        guard let object = try? JSONSerialization.jsonObject(with: self, options: []),
              let data = try? JSONSerialization.data(withJSONObject: object, options: [.prettyPrinted]),
              let prettyPrintedString = String(data: data, encoding: .utf8) else { return nil }
        return prettyPrintedString
    }
}

// MARK: - Array Extensions

extension Array where Element == Message {
    func groupedByDate() -> [(key: Date, value: [Message])] {
        let grouped = Dictionary(grouping: self) { message -> Date in
            Calendar.current.startOfDay(for: message.timestamp)
        }
        return grouped.sorted { $0.key < $1.key }
    }
}

// MARK: - UserDefaults Keys

enum UserDefaultsKey: String {
    case isLoggedIn
    case myJID
    case lastSyncTime
    case notificationsEnabled
    case soundEnabled
    case vibrationEnabled
    case theme
}

extension UserDefaults {
    func set(_ value: Any?, forKey key: UserDefaultsKey) {
        set(value, forKey: key.rawValue)
    }

    func string(forKey key: UserDefaultsKey) -> String? {
        string(forKey: key.rawValue)
    }

    func bool(forKey key: UserDefaultsKey) -> Bool {
        bool(forKey: key.rawValue)
    }
}

// MARK: - Haptic Feedback

enum HapticFeedback {
    static func light() {
        let generator = UIImpactFeedbackGenerator(style: .light)
        generator.impactOccurred()
    }

    static func medium() {
        let generator = UIImpactFeedbackGenerator(style: .medium)
        generator.impactOccurred()
    }

    static func heavy() {
        let generator = UIImpactFeedbackGenerator(style: .heavy)
        generator.impactOccurred()
    }

    static func success() {
        let generator = UINotificationFeedbackGenerator()
        generator.notificationOccurred(.success)
    }

    static func error() {
        let generator = UINotificationFeedbackGenerator()
        generator.notificationOccurred(.error)
    }

    static func selection() {
        let generator = UISelectionFeedbackGenerator()
        generator.selectionChanged()
    }
}
