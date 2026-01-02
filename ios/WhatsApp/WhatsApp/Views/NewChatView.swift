import SwiftUI

struct NewChatView: View {
    @EnvironmentObject var viewModel: ChatViewModel
    @Environment(\.dismiss) var dismiss
    @State private var searchText = ""
    @State private var phoneNumber = ""
    @State private var showNewGroup = false
    @State private var selectedContact: Contact?

    var filteredContacts: [Contact] {
        let sorted = viewModel.contacts.sorted { $0.displayName < $1.displayName }

        if searchText.isEmpty {
            return sorted
        }

        return sorted.filter {
            $0.displayName.localizedCaseInsensitiveContains(searchText) ||
            $0.phoneNumber.contains(searchText)
        }
    }

    var body: some View {
        NavigationStack {
            List {
                // Quick actions
                Section {
                    Button(action: { showNewGroup = true }) {
                        HStack(spacing: 12) {
                            ZStack {
                                Circle()
                                    .fill(Color.whatsappGreen)
                                    .frame(width: 40, height: 40)

                                Image(systemName: "person.3.fill")
                                    .foregroundColor(.white)
                                    .font(.system(size: 14))
                            }

                            Text("New Group")
                                .foregroundColor(.primary)
                        }
                    }

                    Button(action: {}) {
                        HStack(spacing: 12) {
                            ZStack {
                                Circle()
                                    .fill(Color.whatsappGreen)
                                    .frame(width: 40, height: 40)

                                Image(systemName: "person.badge.plus")
                                    .foregroundColor(.white)
                            }

                            Text("New Contact")
                                .foregroundColor(.primary)
                        }
                    }
                }

                // Phone number input
                Section("Start chat with phone number") {
                    HStack {
                        TextField("Enter phone number", text: $phoneNumber)
                            .keyboardType(.phonePad)

                        if !phoneNumber.isEmpty {
                            Button(action: startChatWithNumber) {
                                Image(systemName: "arrow.right.circle.fill")
                                    .foregroundColor(.whatsappGreen)
                                    .font(.title2)
                            }
                        }
                    }
                }

                // Contacts
                Section("Contacts on WhatsApp") {
                    ForEach(filteredContacts) { contact in
                        Button(action: { startChat(with: contact) }) {
                            ContactRowView(contact: contact)
                        }
                        .buttonStyle(.plain)
                    }
                }
            }
            .listStyle(.insetGrouped)
            .navigationTitle("New Chat")
            .navigationBarTitleDisplayMode(.inline)
            .searchable(text: $searchText, prompt: "Search name or number")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        dismiss()
                    }
                }
            }
            .sheet(isPresented: $showNewGroup) {
                NewGroupView()
            }
        }
    }

    private func startChat(with contact: Contact) {
        _ = viewModel.startChat(with: contact)
        dismiss()
    }

    private func startChatWithNumber() {
        Task {
            if await viewModel.startChat(withPhoneNumber: phoneNumber) != nil {
                dismiss()
            }
        }
    }
}

struct NewGroupView: View {
    @EnvironmentObject var viewModel: ChatViewModel
    @Environment(\.dismiss) var dismiss
    @State private var groupName = ""
    @State private var searchText = ""
    @State private var selectedContacts: Set<String> = []
    @State private var step = 1 // 1 = select participants, 2 = set name

    var filteredContacts: [Contact] {
        let nonGroupContacts = viewModel.contacts.filter { !$0.isGroup }
        let sorted = nonGroupContacts.sorted { $0.displayName < $1.displayName }

        if searchText.isEmpty {
            return sorted
        }

        return sorted.filter {
            $0.displayName.localizedCaseInsensitiveContains(searchText)
        }
    }

    var selectedContactsList: [Contact] {
        viewModel.contacts.filter { selectedContacts.contains($0.jid) }
    }

    var body: some View {
        NavigationStack {
            Group {
                if step == 1 {
                    selectParticipantsView
                } else {
                    setGroupInfoView
                }
            }
            .navigationTitle(step == 1 ? "Add Participants" : "New Group")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        dismiss()
                    }
                }

                ToolbarItem(placement: .confirmationAction) {
                    if step == 1 {
                        Button("Next") {
                            withAnimation {
                                step = 2
                            }
                        }
                        .disabled(selectedContacts.isEmpty)
                    } else {
                        Button("Create") {
                            createGroup()
                        }
                        .disabled(groupName.isEmpty)
                    }
                }
            }
        }
    }

    private var selectParticipantsView: some View {
        VStack(spacing: 0) {
            // Selected contacts chips
            if !selectedContacts.isEmpty {
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 8) {
                        ForEach(selectedContactsList) { contact in
                            selectedContactChip(contact)
                        }
                    }
                    .padding(.horizontal)
                    .padding(.vertical, 10)
                }
                .background(Color(.systemGroupedBackground))
            }

            // Contact list
            List {
                ForEach(filteredContacts) { contact in
                    Button(action: { toggleContact(contact) }) {
                        HStack {
                            ContactRowView(contact: contact)

                            Spacer()

                            if selectedContacts.contains(contact.jid) {
                                Image(systemName: "checkmark.circle.fill")
                                    .foregroundColor(.whatsappGreen)
                            } else {
                                Image(systemName: "circle")
                                    .foregroundColor(.secondary)
                            }
                        }
                    }
                    .buttonStyle(.plain)
                }
            }
            .listStyle(.plain)
            .searchable(text: $searchText, prompt: "Search contacts")
        }
    }

    private var setGroupInfoView: some View {
        List {
            Section {
                HStack {
                    // Group icon
                    Button(action: {}) {
                        ZStack {
                            Circle()
                                .fill(Color.whatsappGreen.opacity(0.2))
                                .frame(width: 60, height: 60)

                            Image(systemName: "camera.fill")
                                .foregroundColor(.whatsappGreen)
                        }
                    }

                    TextField("Group Name", text: $groupName)
                        .font(.title3)
                }
                .padding(.vertical, 8)
            }

            Section("Participants: \(selectedContacts.count)") {
                ForEach(selectedContactsList) { contact in
                    ContactRowView(contact: contact)
                }
            }
        }
    }

    private func selectedContactChip(_ contact: Contact) -> some View {
        HStack(spacing: 4) {
            Text(contact.displayName)
                .font(.caption)
                .lineLimit(1)

            Button(action: { toggleContact(contact) }) {
                Image(systemName: "xmark.circle.fill")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 6)
        .background(Color(.systemGray5))
        .cornerRadius(16)
    }

    private func toggleContact(_ contact: Contact) {
        if selectedContacts.contains(contact.jid) {
            selectedContacts.remove(contact.jid)
        } else {
            selectedContacts.insert(contact.jid)
        }
    }

    private func createGroup() {
        Task {
            if await viewModel.createGroup(name: groupName, participants: selectedContactsList) != nil {
                dismiss()
            }
        }
    }
}

#Preview {
    NewChatView()
        .environmentObject({
            let vm = ChatViewModel()
            vm.contacts = Contact.samples
            return vm
        }())
}
