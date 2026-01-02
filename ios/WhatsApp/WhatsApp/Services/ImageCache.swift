import UIKit
import SwiftUI

/// High-performance image cache with memory management and downsampling
actor ImageCache {
    static let shared = ImageCache()

    // In-memory cache with size limit
    private var cache: [String: UIImage] = [:]
    private var cacheSize: Int = 0
    private let maxCacheSize = 100 * 1024 * 1024 // 100 MB

    // Track ongoing downloads to prevent duplicates
    private var ongoingDownloads: [String: Task<UIImage?, Error>] = [:]

    private init() {}

    /// Load image with caching and optional downsampling
    func loadImage(url: URL, maxSize: CGSize? = nil) async throws -> UIImage? {
        let cacheKey = url.absoluteString

        // Check cache first
        if let cachedImage = cache[cacheKey] {
            return cachedImage
        }

        // Check if already downloading
        if let ongoingTask = ongoingDownloads[cacheKey] {
            return try await ongoingTask.value
        }

        // Start new download
        let task = Task<UIImage?, Error> {
            let (data, _) = try await URLSession.shared.data(from: url)

            // Downsample if size specified (significant memory savings)
            let image: UIImage?
            if let maxSize = maxSize {
                image = downsample(imageData: data, to: maxSize)
            } else {
                image = UIImage(data: data)
            }

            guard let finalImage = image else {
                return nil
            }

            // Cache the image
            await cacheImage(finalImage, forKey: cacheKey)

            return finalImage
        }

        ongoingDownloads[cacheKey] = task

        do {
            let image = try await task.value
            ongoingDownloads.removeValue(forKey: cacheKey)
            return image
        } catch {
            ongoingDownloads.removeValue(forKey: cacheKey)
            throw error
        }
    }

    /// Downsample image to reduce memory usage (critical for performance)
    private func downsample(imageData: Data, to targetSize: CGSize) -> UIImage? {
        let imageSourceOptions = [kCGImageSourceShouldCache: false] as CFDictionary
        guard let imageSource = CGImageSourceCreateWithData(imageData as CFData, imageSourceOptions) else {
            return nil
        }

        let maxDimensionInPixels = max(targetSize.width, targetSize.height) * UIScreen.main.scale
        let downsampleOptions = [
            kCGImageSourceCreateThumbnailFromImageAlways: true,
            kCGImageSourceShouldCacheImmediately: true,
            kCGImageSourceCreateThumbnailWithTransform: true,
            kCGImageSourceThumbnailMaxPixelSize: maxDimensionInPixels
        ] as CFDictionary

        guard let downsampledImage = CGImageSourceCreateThumbnailAtIndex(imageSource, 0, downsampleOptions) else {
            return nil
        }

        return UIImage(cgImage: downsampledImage)
    }

    private func cacheImage(_ image: UIImage, forKey key: String) {
        // Estimate image size (width * height * 4 bytes per pixel)
        let imageSize = Int(image.size.width * image.size.height * 4)

        // Remove old images if cache is too large
        if cacheSize + imageSize > maxCacheSize {
            clearOldestImages(toFreeUpSpace: imageSize)
        }

        cache[key] = image
        cacheSize += imageSize
    }

    private func clearOldestImages(toFreeUpSpace neededSpace: Int) {
        var freedSpace = 0
        var keysToRemove: [String] = []

        // Simple FIFO eviction (could be improved with LRU)
        for (key, image) in cache {
            let imageSize = Int(image.size.width * image.size.height * 4)
            keysToRemove.append(key)
            freedSpace += imageSize

            if freedSpace >= neededSpace {
                break
            }
        }

        for key in keysToRemove {
            if let image = cache.removeValue(forKey: key) {
                let imageSize = Int(image.size.width * image.size.height * 4)
                cacheSize -= imageSize
            }
        }
    }

    /// Clear entire cache (useful for low memory situations)
    func clearCache() {
        cache.removeAll()
        cacheSize = 0
    }
}

/// AsyncImage-like view that uses our optimized cache
struct CachedAsyncImage: View {
    let url: URL?
    let maxSize: CGSize?
    let placeholder: () -> AnyView

    @State private var loadedImage: UIImage?
    @State private var isLoading = false
    @State private var loadTask: Task<Void, Never>?

    init(url: URL?, maxSize: CGSize? = nil, @ViewBuilder placeholder: @escaping () -> some View) {
        self.url = url
        self.maxSize = maxSize
        self.placeholder = { AnyView(placeholder()) }
    }

    var body: some View {
        Group {
            if let image = loadedImage {
                Image(uiImage: image)
                    .resizable()
            } else if isLoading {
                ProgressView()
            } else {
                placeholder()
            }
        }
        .onAppear {
            loadImage()
        }
        .onDisappear {
            // Cancel loading if view disappears (important for scrolling performance)
            loadTask?.cancel()
        }
    }

    private func loadImage() {
        guard let url = url else { return }

        isLoading = true

        loadTask = Task {
            do {
                let image = try await ImageCache.shared.loadImage(url: url, maxSize: maxSize)
                if !Task.isCancelled {
                    await MainActor.run {
                        loadedImage = image
                        isLoading = false
                    }
                }
            } catch {
                if !Task.isCancelled {
                    await MainActor.run {
                        isLoading = false
                    }
                }
            }
        }
    }
}
