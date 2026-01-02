import SwiftUI

struct SettingsView: View {
    @EnvironmentObject var viewModel: ChatViewModel
    @State private var showLogoutAlert = false

    var body: some View {
        List {
            // Profile section
            Section {
                NavigationLink(destination: ProfileView()) {
                    HStack(spacing: 15) {
                        ZStack {
                            Circle()
                                .fill(Color.whatsappGreen.opacity(0.2))
                                .frame(width: 60, height: 60)

                            Text(viewModel.myName.prefix(2).uppercased())
                                .font(.title2)
                                .fontWeight(.medium)
                                .foregroundColor(.whatsappGreen)
                        }

                        VStack(alignment: .leading, spacing: 4) {
                            Text(viewModel.myName.isEmpty ? "User" : viewModel.myName)
                                .font(.headline)

                            Text(viewModel.myPhoneNumber.isEmpty ? viewModel.myJID ?? "" : viewModel.myPhoneNumber)
                                .font(.subheadline)
                                .foregroundColor(.secondary)
                        }
                    }
                    .padding(.vertical, 8)
                }
            }

            // Settings sections
            Section {
                settingsRow(icon: "star.fill", iconColor: .yellow, title: "Starred Messages")
                settingsRow(icon: "desktopcomputer", iconColor: .whatsappGreen, title: "Linked Devices")
            }

            Section {
                settingsRow(icon: "key.fill", iconColor: .blue, title: "Account")
                settingsRow(icon: "lock.fill", iconColor: .cyan, title: "Privacy")
                settingsRow(icon: "bubble.left.and.bubble.right.fill", iconColor: .green, title: "Chats")
                settingsRow(icon: "bell.fill", iconColor: .red, title: "Notifications")
                settingsRow(icon: "externaldrive.fill", iconColor: .green, title: "Storage and Data")
            }

            Section {
                settingsRow(icon: "questionmark.circle.fill", iconColor: .blue, title: "Help")
                settingsRow(icon: "heart.fill", iconColor: .red, title: "Tell a Friend")
            }

            // Logout section
            Section {
                Button(action: { showLogoutAlert = true }) {
                    HStack {
                        Image(systemName: "rectangle.portrait.and.arrow.right")
                            .foregroundColor(.red)
                            .frame(width: 28)

                        Text("Log Out")
                            .foregroundColor(.red)
                    }
                }
            }

            // App info
            Section {
                HStack {
                    Spacer()
                    VStack(spacing: 4) {
                        Text("WhatsApp")
                            .font(.headline)
                        Text("Version 1.0.0")
                            .font(.caption)
                            .foregroundColor(.secondary)
                        Text("Powered by whatsmeow")
                            .font(.caption2)
                            .foregroundColor(.secondary)
                    }
                    Spacer()
                }
                .listRowBackground(Color.clear)
            }
        }
        .navigationTitle("Settings")
        .alert("Log Out", isPresented: $showLogoutAlert) {
            Button("Cancel", role: .cancel) {}
            Button("Log Out", role: .destructive) {
                Task {
                    await viewModel.logout()
                }
            }
        } message: {
            Text("Are you sure you want to log out? You will need to scan the QR code again to log in.")
        }
    }

    private func settingsRow(icon: String, iconColor: Color, title: String) -> some View {
        NavigationLink(destination: Text(title)) {
            HStack(spacing: 15) {
                ZStack {
                    RoundedRectangle(cornerRadius: 6)
                        .fill(iconColor)
                        .frame(width: 28, height: 28)

                    Image(systemName: icon)
                        .font(.system(size: 14))
                        .foregroundColor(.white)
                }

                Text(title)
            }
        }
    }
}

struct ProfileView: View {
    @EnvironmentObject var viewModel: ChatViewModel
    @State private var name = ""
    @State private var about = "Hey there! I am using WhatsApp."
    @State private var showImagePicker = false

    var body: some View {
        List {
            // Profile photo
            Section {
                HStack {
                    Spacer()
                    Button(action: { showImagePicker = true }) {
                        ZStack(alignment: .bottomTrailing) {
                            Circle()
                                .fill(Color.whatsappGreen.opacity(0.2))
                                .frame(width: 120, height: 120)
                                .overlay(
                                    Text(viewModel.myName.prefix(2).uppercased())
                                        .font(.largeTitle)
                                        .fontWeight(.medium)
                                        .foregroundColor(.whatsappGreen)
                                )

                            ZStack {
                                Circle()
                                    .fill(Color.whatsappGreen)
                                    .frame(width: 36, height: 36)

                                Image(systemName: "camera.fill")
                                    .font(.system(size: 16))
                                    .foregroundColor(.white)
                            }
                        }
                    }
                    Spacer()
                }
                .listRowBackground(Color.clear)
            }

            // Name
            Section {
                HStack {
                    Image(systemName: "person.fill")
                        .foregroundColor(.secondary)
                        .frame(width: 24)

                    VStack(alignment: .leading, spacing: 4) {
                        Text("Name")
                            .font(.caption)
                            .foregroundColor(.secondary)

                        TextField("Your name", text: $name)
                    }
                }
            } footer: {
                Text("This is not your username or PIN. This name will be visible to your WhatsApp contacts.")
            }

            // About
            Section {
                HStack {
                    Image(systemName: "info.circle.fill")
                        .foregroundColor(.secondary)
                        .frame(width: 24)

                    VStack(alignment: .leading, spacing: 4) {
                        Text("About")
                            .font(.caption)
                            .foregroundColor(.secondary)

                        TextField("About", text: $about)
                    }
                }
            }

            // Phone
            Section {
                HStack {
                    Image(systemName: "phone.fill")
                        .foregroundColor(.secondary)
                        .frame(width: 24)

                    VStack(alignment: .leading, spacing: 4) {
                        Text("Phone")
                            .font(.caption)
                            .foregroundColor(.secondary)

                        Text(viewModel.myPhoneNumber.isEmpty ? viewModel.myJID ?? "Unknown" : viewModel.myPhoneNumber)
                    }
                }
            }
        }
        .navigationTitle("Profile")
        .navigationBarTitleDisplayMode(.inline)
        .onAppear {
            name = viewModel.myName
        }
        .sheet(isPresented: $showImagePicker) {
            ImagePicker { _ in
                // Handle profile photo update
            }
        }
    }
}

#Preview {
    NavigationStack {
        SettingsView()
            .environmentObject({
                let vm = ChatViewModel()
                vm.myName = "John Doe"
                vm.myPhoneNumber = "+1 234 567 890"
                return vm
            }())
    }
}
