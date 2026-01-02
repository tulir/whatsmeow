import SwiftUI

struct ChatListView: View {
    @EnvironmentObject var viewModel: ChatViewModel
    @State private var showNewChat = false
    @State private var showSearch = false

    var body: some View {
        List {
            // Pinned chats section
            if !viewModel.pinnedChats.isEmpty {
                Section {
                    ForEach(viewModel.pinnedChats) { chat in
                        NavigationLink(destination: ChatView(chat: chat)) {
                            ChatRowView(chat: chat)
                        }
                        .swipeActions(edge: .trailing, allowsFullSwipe: true) {
                            deleteButton(for: chat)
                            archiveButton(for: chat)
                        }
                        .swipeActions(edge: .leading) {
                            pinButton(for: chat)
                            muteButton(for: chat)
                        }
                    }
                } header: {
                    Text("Pinned")
                        .font(.caption)
                        .foregroundColor(.secondary)
                }
            }

            // Regular chats section
            Section {
                ForEach(viewModel.regularChats) { chat in
                    NavigationLink(destination: ChatView(chat: chat)) {
                        ChatRowView(chat: chat)
                    }
                    .swipeActions(edge: .trailing, allowsFullSwipe: true) {
                        deleteButton(for: chat)
                        archiveButton(for: chat)
                    }
                    .swipeActions(edge: .leading) {
                        pinButton(for: chat)
                        muteButton(for: chat)
                    }
                }
            }
        }
        .listStyle(.plain)
        .navigationTitle("Chats")
        .searchable(text: $viewModel.searchText, prompt: "Search chats")
        .toolbar {
            ToolbarItem(placement: .topBarLeading) {
                EditButton()
            }

            ToolbarItem(placement: .topBarTrailing) {
                Button(action: { showNewChat = true }) {
                    Image(systemName: "square.and.pencil")
                }
            }
        }
        .sheet(isPresented: $showNewChat) {
            NewChatView()
        }
        .refreshable {
            // Refresh chats
            try? await Task.sleep(nanoseconds: 1_000_000_000)
        }
        .overlay {
            if viewModel.chats.isEmpty {
                emptyStateView
            }
        }
    }

    private var emptyStateView: some View {
        VStack(spacing: 20) {
            Image(systemName: "bubble.left.and.bubble.right")
                .font(.system(size: 60))
                .foregroundColor(.secondary)

            Text("No Chats Yet")
                .font(.title2)
                .fontWeight(.semibold)

            Text("Start a new conversation by tapping the compose button")
                .font(.subheadline)
                .foregroundColor(.secondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal, 40)

            Button(action: { showNewChat = true }) {
                Text("Start Chat")
                    .font(.headline)
                    .foregroundColor(.white)
                    .padding(.horizontal, 30)
                    .padding(.vertical, 12)
                    .background(Color.whatsappGreen)
                    .cornerRadius(25)
            }
        }
    }

    private func deleteButton(for chat: Chat) -> some View {
        Button(role: .destructive) {
            withAnimation {
                viewModel.deleteChat(chat)
            }
        } label: {
            Label("Delete", systemImage: "trash")
        }
    }

    private func archiveButton(for chat: Chat) -> some View {
        Button {
            withAnimation {
                viewModel.archiveChat(chat)
            }
        } label: {
            Label("Archive", systemImage: "archivebox")
        }
        .tint(.gray)
    }

    private func pinButton(for chat: Chat) -> some View {
        Button {
            withAnimation {
                viewModel.pinChat(chat)
            }
        } label: {
            Label(chat.isPinned ? "Unpin" : "Pin", systemImage: chat.isPinned ? "pin.slash" : "pin")
        }
        .tint(.orange)
    }

    private func muteButton(for chat: Chat) -> some View {
        Button {
            withAnimation {
                viewModel.muteChat(chat)
            }
        } label: {
            Label(chat.isMuted ? "Unmute" : "Mute", systemImage: chat.isMuted ? "bell" : "bell.slash")
        }
        .tint(.indigo)
    }
}

struct ChatRowView: View {
    let chat: Chat

    var body: some View {
        HStack(spacing: 12) {
            // Avatar
            ZStack {
                Circle()
                    .fill(Color.whatsappGreen.opacity(0.2))
                    .frame(width: 56, height: 56)

                if chat.isGroup {
                    Image(systemName: "person.3.fill")
                        .foregroundColor(.whatsappGreen)
                } else {
                    Text(chat.initials)
                        .font(.title3)
                        .fontWeight(.medium)
                        .foregroundColor(.whatsappGreen)
                }
            }

            // Content
            VStack(alignment: .leading, spacing: 4) {
                HStack {
                    Text(chat.displayName)
                        .font(.headline)
                        .lineLimit(1)

                    Spacer()

                    Text(chat.lastMessageTimeFormatted)
                        .font(.caption)
                        .foregroundColor(chat.unreadCount > 0 ? .whatsappGreen : .secondary)
                }

                HStack {
                    // Last message with icons
                    HStack(spacing: 4) {
                        if chat.isMuted {
                            Image(systemName: "speaker.slash.fill")
                                .font(.caption2)
                                .foregroundColor(.secondary)
                        }

                        Text(chat.lastMessage)
                            .font(.subheadline)
                            .foregroundColor(.secondary)
                            .lineLimit(1)
                    }

                    Spacer()

                    // Unread badge
                    if chat.unreadCount > 0 {
                        Text(chat.unreadCount > 99 ? "99+" : "\(chat.unreadCount)")
                            .font(.caption2)
                            .fontWeight(.bold)
                            .foregroundColor(.white)
                            .padding(.horizontal, 8)
                            .padding(.vertical, 4)
                            .background(Color.whatsappGreen)
                            .clipShape(Capsule())
                    }

                    if chat.isPinned {
                        Image(systemName: "pin.fill")
                            .font(.caption)
                            .foregroundColor(.secondary)
                    }
                }
            }
        }
        .padding(.vertical, 4)
    }
}

#Preview {
    NavigationStack {
        ChatListView()
            .environmentObject({
                let vm = ChatViewModel()
                vm.chats = Chat.samples
                return vm
            }())
    }
}
