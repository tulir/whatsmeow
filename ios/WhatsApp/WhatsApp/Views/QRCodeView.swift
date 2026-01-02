import SwiftUI
import CoreImage.CIFilterBuiltins

struct QRCodeView: View {
    let qrCode: String
    @EnvironmentObject var viewModel: ChatViewModel
    @State private var isRefreshing = false

    var body: some View {
        VStack(spacing: 30) {
            Spacer()

            Text("Scan to Log In")
                .font(.title)
                .fontWeight(.bold)

            Text("Open WhatsApp on your phone, go to Settings > Linked Devices > Link a Device")
                .font(.subheadline)
                .foregroundColor(.secondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal, 40)

            // QR Code Image
            ZStack {
                RoundedRectangle(cornerRadius: 20)
                    .fill(Color.white)
                    .shadow(color: .black.opacity(0.1), radius: 10)

                if let qrImage = generateQRCode(from: qrCode) {
                    Image(uiImage: qrImage)
                        .interpolation(.none)
                        .resizable()
                        .scaledToFit()
                        .padding(20)
                } else {
                    ProgressView()
                }
            }
            .frame(width: 280, height: 280)

            // Instructions
            VStack(alignment: .leading, spacing: 15) {
                InstructionRow(number: 1, text: "Open WhatsApp on your phone")
                InstructionRow(number: 2, text: "Tap Menu or Settings and select Linked Devices")
                InstructionRow(number: 3, text: "Point your phone at this screen to capture the QR code")
            }
            .padding(.horizontal, 40)

            Spacer()

            // Refresh button
            Button(action: refreshQRCode) {
                HStack {
                    if isRefreshing {
                        ProgressView()
                            .scaleEffect(0.8)
                    } else {
                        Image(systemName: "arrow.clockwise")
                    }
                    Text("Refresh QR Code")
                }
                .font(.subheadline)
                .foregroundColor(.whatsappGreen)
            }
            .disabled(isRefreshing)
            .padding(.bottom, 30)
        }
        .background(Color(.systemGroupedBackground))
    }

    private func generateQRCode(from string: String) -> UIImage? {
        let context = CIContext()
        let filter = CIFilter.qrCodeGenerator()

        guard let data = string.data(using: .utf8) else { return nil }

        filter.setValue(data, forKey: "inputMessage")
        filter.setValue("M", forKey: "inputCorrectionLevel")

        guard let outputImage = filter.outputImage else { return nil }

        // Scale up the QR code
        let scale = 10.0
        let transform = CGAffineTransform(scaleX: scale, y: scale)
        let scaledImage = outputImage.transformed(by: transform)

        guard let cgImage = context.createCGImage(scaledImage, from: scaledImage.extent) else {
            return nil
        }

        return UIImage(cgImage: cgImage)
    }

    private func refreshQRCode() {
        isRefreshing = true
        viewModel.connect()

        DispatchQueue.main.asyncAfter(deadline: .now() + 2) {
            isRefreshing = false
        }
    }
}

struct InstructionRow: View {
    let number: Int
    let text: String

    var body: some View {
        HStack(alignment: .top, spacing: 15) {
            ZStack {
                Circle()
                    .fill(Color.whatsappGreen)
                    .frame(width: 24, height: 24)

                Text("\(number)")
                    .font(.caption)
                    .fontWeight(.bold)
                    .foregroundColor(.white)
            }

            Text(text)
                .font(.subheadline)
                .foregroundColor(.primary)
        }
    }
}

#Preview {
    QRCodeView(qrCode: "1@TestQRCode,SampleData,123456")
        .environmentObject(ChatViewModel())
}
