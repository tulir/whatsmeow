import SwiftUI

struct MessageBubbleView: View {
    let message: Message
    let isGroupChat: Bool

    var body: some View {
        HStack(alignment: .bottom, spacing: 0) {
            if message.isFromMe {
                Spacer(minLength: 60)
            }

            VStack(alignment: message.isFromMe ? .trailing : .leading, spacing: 2) {
                // Sender name (for group chats)
                if isGroupChat && !message.isFromMe {
                    Text(message.senderName)
                        .font(.caption)
                        .fontWeight(.semibold)
                        .foregroundColor(senderColor(for: message.senderJID))
                        .padding(.leading, 12)
                }

                // Message bubble
                HStack(alignment: .bottom, spacing: 4) {
                    VStack(alignment: .leading, spacing: 4) {
                        // Quoted message
                        if let quotedText = message.quotedText {
                            QuotedMessageView(text: quotedText)
                        }

                        // Media content
                        if message.hasMedia {
                            MediaPreviewView(message: message)
                        }

                        // Text content
                        if !message.text.isEmpty {
                            Text(message.text)
                                .font(.body)
                                .foregroundColor(.primary)
                        }
                    }

                    // Timestamp and status
                    HStack(spacing: 2) {
                        Text(message.timestamp.messageTimeString)
                            .font(.caption2)
                            .foregroundColor(.secondary)

                        if message.isFromMe {
                            messageStatusIcon
                        }
                    }
                }
                .padding(.horizontal, 12)
                .padding(.vertical, 8)
                .background(message.isFromMe ? Color.messageBubbleOut : Color.messageBubbleIn)
                .cornerRadius(16, corners: message.isFromMe ? [.topLeft, .topRight, .bottomLeft] : [.topLeft, .topRight, .bottomRight])
                .shadow(color: .black.opacity(0.05), radius: 1, y: 1)
            }

            if !message.isFromMe {
                Spacer(minLength: 60)
            }
        }
        .padding(.horizontal, 4)
        .padding(.vertical, 1)
    }

    @ViewBuilder
    private var messageStatusIcon: some View {
        switch message.status {
        case .sending:
            Image(systemName: "clock")
                .font(.caption2)
                .foregroundColor(.secondary)
        case .sent:
            Image(systemName: "checkmark")
                .font(.caption2)
                .foregroundColor(.secondary)
        case .delivered:
            Image(systemName: "checkmark")
                .font(.caption2)
                .foregroundColor(.secondary)
                .overlay(
                    Image(systemName: "checkmark")
                        .font(.caption2)
                        .foregroundColor(.secondary)
                        .offset(x: 4)
                )
        case .read:
            Image(systemName: "checkmark")
                .font(.caption2)
                .foregroundColor(.whatsappBlue)
                .overlay(
                    Image(systemName: "checkmark")
                        .font(.caption2)
                        .foregroundColor(.whatsappBlue)
                        .offset(x: 4)
                )
        case .failed:
            Image(systemName: "exclamationmark.circle")
                .font(.caption2)
                .foregroundColor(.red)
        }
    }

    private func senderColor(for jid: String) -> Color {
        // Generate consistent color based on JID
        let colors: [Color] = [
            .red, .orange, .purple, .blue, .pink, .teal, .indigo
        ]
        let hash = abs(jid.hashValue)
        return colors[hash % colors.count]
    }
}

struct QuotedMessageView: View {
    let text: String

    var body: some View {
        HStack(spacing: 0) {
            Rectangle()
                .fill(Color.whatsappGreen)
                .frame(width: 4)

            Text(text)
                .font(.caption)
                .foregroundColor(.secondary)
                .lineLimit(2)
                .padding(.horizontal, 8)
                .padding(.vertical, 4)
        }
        .background(Color.black.opacity(0.05))
        .cornerRadius(4)
    }
}

struct MediaPreviewView: View {
    let message: Message
    @State private var loadedImage: UIImage?
    @State private var isLoading = false

    var body: some View {
        Group {
            switch message.mediaType {
            case .image:
                imagePreview
            case .video:
                videoPreview
            case .audio:
                audioPreview
            case .document:
                documentPreview
            case .sticker:
                stickerPreview
            case .none:
                EmptyView()
            }
        }
        .onAppear {
            if message.mediaType == .image {
                loadMediaImage()
            }
        }
    }

    private var imagePreview: some View {
        ZStack {
            RoundedRectangle(cornerRadius: 8)
                .fill(Color.gray.opacity(0.2))
                .frame(width: 200, height: 150)

            if let image = loadedImage {
                Image(uiImage: image)
                    .resizable()
                    .scaledToFill()
                    .frame(width: 200, height: 150)
                    .clipped()
                    .cornerRadius(8)
            } else if isLoading {
                ProgressView()
            } else {
                Image(systemName: "photo")
                    .font(.largeTitle)
                    .foregroundColor(.secondary)
            }
        }
    }

    private func loadMediaImage() {
        guard let urlString = message.mediaURL, !urlString.isEmpty else { return }
        guard let url = URL(string: urlString) else { return }

        isLoading = true

        // Load image on background thread
        Task.detached(priority: .userInitiated) {
            do {
                let (data, _) = try await URLSession.shared.data(from: url)
                if let image = UIImage(data: data) {
                    await MainActor.run {
                        loadedImage = image
                        isLoading = false
                    }
                }
            } catch {
                print("Failed to load media image: \(error)")
                await MainActor.run {
                    isLoading = false
                }
            }
        }
    }

    private var videoPreview: some View {
        ZStack {
            RoundedRectangle(cornerRadius: 8)
                .fill(Color.gray.opacity(0.2))
                .frame(width: 200, height: 150)

            Image(systemName: "play.circle.fill")
                .font(.system(size: 50))
                .foregroundColor(.white)
                .shadow(radius: 2)
        }
    }

    private var audioPreview: some View {
        HStack(spacing: 10) {
            Image(systemName: "play.circle.fill")
                .font(.title)
                .foregroundColor(.whatsappGreen)

            // Waveform placeholder
            HStack(spacing: 2) {
                ForEach(0..<20, id: \.self) { _ in
                    RoundedRectangle(cornerRadius: 1)
                        .fill(Color.secondary)
                        .frame(width: 2, height: CGFloat.random(in: 5...20))
                }
            }

            Text("0:30")
                .font(.caption)
                .foregroundColor(.secondary)
        }
        .padding(8)
        .frame(width: 200)
    }

    private var documentPreview: some View {
        HStack(spacing: 10) {
            Image(systemName: "doc.fill")
                .font(.title)
                .foregroundColor(.red)

            VStack(alignment: .leading, spacing: 2) {
                Text(message.text.isEmpty ? "Document" : message.text)
                    .font(.subheadline)
                    .lineLimit(1)

                Text("PDF")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
        }
        .padding(10)
        .background(Color.gray.opacity(0.1))
        .cornerRadius(8)
    }

    private var stickerPreview: some View {
        ZStack {
            RoundedRectangle(cornerRadius: 8)
                .fill(Color.clear)
                .frame(width: 120, height: 120)

            Image(systemName: "face.smiling")
                .font(.system(size: 60))
                .foregroundColor(.yellow)
        }
    }
}

// Custom corner radius modifier
extension View {
    func cornerRadius(_ radius: CGFloat, corners: UIRectCorner) -> some View {
        clipShape(RoundedCorner(radius: radius, corners: corners))
    }
}

struct RoundedCorner: Shape {
    var radius: CGFloat = .infinity
    var corners: UIRectCorner = .allCorners

    func path(in rect: CGRect) -> Path {
        let path = UIBezierPath(
            roundedRect: rect,
            byRoundingCorners: corners,
            cornerRadii: CGSize(width: radius, height: radius)
        )
        return Path(path.cgPath)
    }
}

#Preview {
    VStack(spacing: 10) {
        MessageBubbleView(message: Message.sample, isGroupChat: false)
        MessageBubbleView(message: Message.sampleOutgoing, isGroupChat: false)
    }
    .padding()
    .background(Color.chatBackground)
}
