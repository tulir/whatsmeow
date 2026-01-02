import SwiftUI

struct ContentView: View {
    @EnvironmentObject var viewModel: ChatViewModel

    var body: some View {
        Group {
            if viewModel.isLoggedIn {
                MainTabView()
            } else if viewModel.isConnecting {
                LoadingView()
            } else if let qrCode = viewModel.currentQRCode {
                QRCodeView(qrCode: qrCode)
            } else {
                WelcomeView()
            }
        }
        .onAppear {
            viewModel.initialize()
        }
    }
}

struct MainTabView: View {
    @EnvironmentObject var viewModel: ChatViewModel

    var body: some View {
        TabView {
            NavigationStack {
                ChatListView()
            }
            .tabItem {
                Label("Chats", systemImage: "message.fill")
            }

            NavigationStack {
                ContactsView()
            }
            .tabItem {
                Label("Contacts", systemImage: "person.2.fill")
            }

            NavigationStack {
                SettingsView()
            }
            .tabItem {
                Label("Settings", systemImage: "gear")
            }
        }
        .tint(.green)
    }
}

struct WelcomeView: View {
    @EnvironmentObject var viewModel: ChatViewModel

    var body: some View {
        VStack(spacing: 30) {
            Spacer()

            Image(systemName: "message.fill")
                .resizable()
                .scaledToFit()
                .frame(width: 100, height: 100)
                .foregroundColor(.green)

            Text("WhatsApp")
                .font(.largeTitle)
                .fontWeight(.bold)

            Text("Simple. Secure. Reliable messaging.")
                .font(.subheadline)
                .foregroundColor(.secondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal)

            Spacer()

            Button(action: {
                viewModel.connect()
            }) {
                Text("Connect with QR Code")
                    .font(.headline)
                    .foregroundColor(.white)
                    .frame(maxWidth: .infinity)
                    .padding()
                    .background(Color.green)
                    .cornerRadius(12)
            }
            .padding(.horizontal, 40)

            Text("Scan the QR code on your phone to link this device")
                .font(.caption)
                .foregroundColor(.secondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal)

            Spacer()
                .frame(height: 50)
        }
    }
}

struct LoadingView: View {
    var body: some View {
        VStack(spacing: 20) {
            ProgressView()
                .scaleEffect(1.5)

            Text("Connecting...")
                .font(.headline)
                .foregroundColor(.secondary)
        }
    }
}

#Preview {
    ContentView()
        .environmentObject(ChatViewModel())
}
