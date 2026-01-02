import SwiftUI

struct ChatView: View {
    let chat: Chat
    @EnvironmentObject var viewModel: ChatViewModel
    @State private var messageText = ""
    @State private var showImagePicker = false
    @State private var showAttachmentMenu = false
    @FocusState private var isTextFieldFocused: Bool

    // Cache grouped messages to avoid expensive grouping on every render
    @State private var groupedMessages: [(key: Date, value: [Message])] = []
    @State private var lastMessageCount = 0

    var messages: [Message] {
        viewModel.getMessages(for: chat.jid)
    }

    var body: some View {
        VStack(spacing: 0) {
            // Messages list
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(spacing: 2) {
                        ForEach(groupedMessages, id: \.key) { date, dayMessages in
                            // Date header
                            DateHeaderView(date: date)
                                .padding(.top, 10)

                            // Messages for this date
                            ForEach(dayMessages) { message in
                                MessageBubbleView(message: message, isGroupChat: chat.isGroup)
                                    .id(message.id)
                            }
                        }
                    }
                    .padding(.horizontal, 8)
                    .padding(.bottom, 10)
                }
                .background(Color.chatBackground)
                .onChange(of: messages.count) { _, newCount in
                    // Only regroup if message count changed (performance optimization)
                    if newCount != lastMessageCount {
                        updateGroupedMessages()
                        lastMessageCount = newCount
                    }

                    if let lastMessage = messages.last {
                        withAnimation(.easeOut(duration: 0.2)) {
                            proxy.scrollTo(lastMessage.id, anchor: .bottom)
                        }
                    }
                }
                .onAppear {
                    // Initial grouping on appear
                    updateGroupedMessages()
                    lastMessageCount = messages.count

                    if let lastMessage = messages.last {
                        proxy.scrollTo(lastMessage.id, anchor: .bottom)
                    }
                }
            }

            // Input bar
            inputBar
        }
        .navigationTitle(chat.displayName)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .principal) {
                VStack(spacing: 0) {
                    Text(chat.displayName)
                        .font(.headline)

                    if chat.isGroup, let count = chat.participantCount {
                        Text("\(count) participants")
                            .font(.caption)
                            .foregroundColor(.secondary)
                    }
                }
            }

            ToolbarItem(placement: .topBarTrailing) {
                HStack(spacing: 15) {
                    Button(action: {}) {
                        Image(systemName: "video")
                    }

                    Button(action: {}) {
                        Image(systemName: "phone")
                    }
                }
            }
        }
        .onAppear {
            Task {
                await viewModel.markAsRead(chatJID: chat.jid)
            }
        }
        .onDisappear {
            viewModel.sendTyping(to: chat.jid, isTyping: false)
        }
        .sheet(isPresented: $showImagePicker) {
            ImagePicker { image in
                if let imageData = image.jpegData(compressionQuality: 0.8) {
                    Task {
                        await viewModel.sendImage(to: chat.jid, imageData: imageData)
                    }
                }
            }
        }
        .confirmationDialog("Send", isPresented: $showAttachmentMenu) {
            Button("Photo & Video Library") {
                showImagePicker = true
            }
            Button("Camera") {
                // Open camera
            }
            Button("Document") {
                // Open document picker
            }
            Button("Contact") {
                // Share contact
            }
            Button("Location") {
                // Share location
            }
            Button("Cancel", role: .cancel) {}
        }
    }

    private var inputBar: some View {
        HStack(spacing: 8) {
            // Attachment button
            Button(action: { showAttachmentMenu = true }) {
                Image(systemName: "plus")
                    .font(.title2)
                    .foregroundColor(.secondary)
            }

            // Text field
            HStack(spacing: 8) {
                TextField("Message", text: $messageText, axis: .vertical)
                    .lineLimit(1...5)
                    .focused($isTextFieldFocused)
                    .onChange(of: messageText) { _, newValue in
                        viewModel.sendTyping(to: chat.jid, isTyping: !newValue.isEmpty)
                    }

                // Emoji button
                Button(action: {}) {
                    Image(systemName: "face.smiling")
                        .foregroundColor(.secondary)
                }
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 8)
            .background(Color(.systemGray6))
            .cornerRadius(20)

            // Send/Voice button
            Button(action: sendMessage) {
                Image(systemName: messageText.isEmpty ? "mic.fill" : "arrow.up.circle.fill")
                    .font(.title)
                    .foregroundColor(messageText.isEmpty ? .secondary : .whatsappGreen)
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(Color(.systemBackground))
    }

    private func sendMessage() {
        guard !messageText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            return
        }

        let text = messageText
        messageText = ""

        Task {
            await viewModel.sendMessage(to: chat.jid, text: text)
        }
    }

    /// Update grouped messages cache (only called when messages change)
    private func updateGroupedMessages() {
        groupedMessages = messages.groupedByDate()
    }
}

struct DateHeaderView: View {
    let date: Date

    var body: some View {
        Text(formattedDate)
            .font(.caption)
            .fontWeight(.medium)
            .foregroundColor(.secondary)
            .padding(.horizontal, 12)
            .padding(.vertical, 6)
            .background(Color.white.opacity(0.9))
            .cornerRadius(8)
    }

    private var formattedDate: String {
        let calendar = Calendar.current

        if calendar.isDateInToday(date) {
            return "Today"
        } else if calendar.isDateInYesterday(date) {
            return "Yesterday"
        } else {
            let formatter = DateFormatter()
            formatter.dateFormat = "EEEE, MMM d, yyyy"
            return formatter.string(from: date)
        }
    }
}

#Preview {
    NavigationStack {
        ChatView(chat: Chat.sample)
            .environmentObject({
                let vm = ChatViewModel()
                vm.messages[Chat.sample.jid] = [Message.sample, Message.sampleOutgoing]
                return vm
            }())
    }
}
