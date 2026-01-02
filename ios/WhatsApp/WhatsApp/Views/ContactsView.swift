import SwiftUI

struct ContactsView: View {
    @EnvironmentObject var viewModel: ChatViewModel
    @State private var searchText = ""
    @State private var selectedContact: Contact?
    @State private var showNewContact = false

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

    var groupedContacts: [(String, [Contact])] {
        let grouped = Dictionary(grouping: filteredContacts) { contact -> String in
            String(contact.displayName.prefix(1)).uppercased()
        }
        return grouped.sorted { $0.key < $1.key }
    }

    var body: some View {
        List {
            // New contact button
            Section {
                Button(action: { showNewContact = true }) {
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

                Button(action: {}) {
                    HStack(spacing: 12) {
                        ZStack {
                            Circle()
                                .fill(Color.whatsappGreen)
                                .frame(width: 40, height: 40)

                            Image(systemName: "person.2.fill")
                                .foregroundColor(.white)
                        }

                        Text("New Group")
                            .foregroundColor(.primary)
                    }
                }
            }

            // Contacts by letter
            ForEach(groupedContacts, id: \.0) { letter, contacts in
                Section {
                    ForEach(contacts) { contact in
                        NavigationLink(destination: contactDetail(for: contact)) {
                            ContactRowView(contact: contact)
                        }
                    }
                } header: {
                    Text(letter)
                }
            }
        }
        .listStyle(.plain)
        .navigationTitle("Contacts")
        .searchable(text: $searchText, prompt: "Search contacts")
        .refreshable {
            await viewModel.refreshContacts()
        }
        .overlay {
            if viewModel.contacts.isEmpty {
                emptyStateView
            }
        }
        .sheet(isPresented: $showNewContact) {
            NewContactView()
        }
    }

    private var emptyStateView: some View {
        VStack(spacing: 20) {
            Image(systemName: "person.crop.circle.badge.questionmark")
                .font(.system(size: 60))
                .foregroundColor(.secondary)

            Text("No Contacts")
                .font(.title2)
                .fontWeight(.semibold)

            Text("Your WhatsApp contacts will appear here")
                .font(.subheadline)
                .foregroundColor(.secondary)
                .multilineTextAlignment(.center)

            Button(action: { showNewContact = true }) {
                Text("Add Contact")
                    .font(.headline)
                    .foregroundColor(.white)
                    .padding(.horizontal, 30)
                    .padding(.vertical, 12)
                    .background(Color.whatsappGreen)
                    .cornerRadius(25)
            }
        }
        .padding()
    }

    private func contactDetail(for contact: Contact) -> some View {
        ContactDetailView(contact: contact)
    }
}

struct ContactRowView: View {
    let contact: Contact

    var body: some View {
        HStack(spacing: 12) {
            // Avatar
            ZStack {
                Circle()
                    .fill(Color.whatsappGreen.opacity(0.2))
                    .frame(width: 44, height: 44)

                if contact.isGroup {
                    Image(systemName: "person.3.fill")
                        .foregroundColor(.whatsappGreen)
                } else {
                    Text(contact.initials)
                        .font(.headline)
                        .foregroundColor(.whatsappGreen)
                }
            }

            VStack(alignment: .leading, spacing: 2) {
                Text(contact.displayName)
                    .font(.body)

                if !contact.phoneNumber.isEmpty {
                    Text(contact.formattedPhoneNumber)
                        .font(.caption)
                        .foregroundColor(.secondary)
                }
            }
        }
        .padding(.vertical, 4)
    }
}

struct ContactDetailView: View {
    let contact: Contact
    @EnvironmentObject var viewModel: ChatViewModel
    @Environment(\.dismiss) var dismiss

    var body: some View {
        List {
            // Header
            Section {
                VStack(spacing: 15) {
                    ZStack {
                        Circle()
                            .fill(Color.whatsappGreen.opacity(0.2))
                            .frame(width: 100, height: 100)

                        Text(contact.initials)
                            .font(.largeTitle)
                            .fontWeight(.medium)
                            .foregroundColor(.whatsappGreen)
                    }

                    Text(contact.displayName)
                        .font(.title2)
                        .fontWeight(.semibold)

                    if !contact.phoneNumber.isEmpty {
                        Text(contact.formattedPhoneNumber)
                            .font(.subheadline)
                            .foregroundColor(.secondary)
                    }

                    // Action buttons
                    HStack(spacing: 30) {
                        actionButton(icon: "message.fill", title: "Message") {
                            startChat()
                        }
                        actionButton(icon: "phone.fill", title: "Audio") {}
                        actionButton(icon: "video.fill", title: "Video") {}
                    }
                    .padding(.top, 10)
                }
                .frame(maxWidth: .infinity)
                .padding(.vertical)
            }

            // Info section
            Section("Info") {
                if !contact.phoneNumber.isEmpty {
                    HStack {
                        Text("Phone")
                            .foregroundColor(.secondary)
                        Spacer()
                        Text(contact.formattedPhoneNumber)
                    }
                }

                if !contact.pushName.isEmpty {
                    HStack {
                        Text("Push Name")
                            .foregroundColor(.secondary)
                        Spacer()
                        Text(contact.pushName)
                    }
                }
            }

            // Actions
            Section {
                Button(action: {}) {
                    HStack {
                        Image(systemName: "star")
                            .foregroundColor(.yellow)
                        Text("Add to Favorites")
                    }
                }

                Button(role: .destructive, action: {}) {
                    HStack {
                        Image(systemName: "hand.raised")
                        Text("Block Contact")
                    }
                }
            }
        }
        .navigationTitle("Contact Info")
        .navigationBarTitleDisplayMode(.inline)
    }

    private func actionButton(icon: String, title: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            VStack(spacing: 8) {
                ZStack {
                    Circle()
                        .fill(Color.whatsappGreen.opacity(0.1))
                        .frame(width: 50, height: 50)

                    Image(systemName: icon)
                        .foregroundColor(.whatsappGreen)
                }

                Text(title)
                    .font(.caption)
                    .foregroundColor(.primary)
            }
        }
    }

    private func startChat() {
        _ = viewModel.startChat(with: contact)
        dismiss()
    }
}

struct NewContactView: View {
    @Environment(\.dismiss) var dismiss
    @State private var name = ""
    @State private var phoneNumber = ""

    var body: some View {
        NavigationStack {
            Form {
                Section("Contact Info") {
                    TextField("Name", text: $name)
                    TextField("Phone Number", text: $phoneNumber)
                        .keyboardType(.phonePad)
                }
            }
            .navigationTitle("New Contact")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        dismiss()
                    }
                }

                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") {
                        // Save contact
                        dismiss()
                    }
                    .disabled(name.isEmpty || phoneNumber.isEmpty)
                }
            }
        }
    }
}

#Preview {
    NavigationStack {
        ContactsView()
            .environmentObject({
                let vm = ChatViewModel()
                vm.contacts = Contact.samples
                return vm
            }())
    }
}
