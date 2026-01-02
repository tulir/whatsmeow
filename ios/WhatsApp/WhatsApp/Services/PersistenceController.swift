import CoreData

// MARK: - CoreData Entities (defined programmatically)

@objc(MessageEntity)
class MessageEntity: NSManagedObject {
    @NSManaged var id: String?
    @NSManaged var chatJID: String?
    @NSManaged var senderJID: String?
    @NSManaged var senderName: String?
    @NSManaged var text: String?
    @NSManaged var timestamp: Date?
    @NSManaged var isFromMe: Bool
    @NSManaged var isGroup: Bool
    @NSManaged var mediaType: String?
    @NSManaged var mediaURL: String?
    @NSManaged var mediaCaption: String?
    @NSManaged var quotedID: String?
    @NSManaged var quotedText: String?
    @NSManaged var status: String?

    func toMessage() -> Message {
        Message(
            id: id ?? "",
            chatJID: chatJID ?? "",
            senderJID: senderJID ?? "",
            senderName: senderName ?? "",
            text: text ?? "",
            timestamp: timestamp ?? Date(),
            isFromMe: isFromMe,
            isGroup: isGroup,
            mediaType: Message.MediaType(rawValue: mediaType ?? "") ?? .none,
            mediaURL: mediaURL,
            mediaCaption: mediaCaption,
            quotedID: quotedID,
            quotedText: quotedText,
            status: Message.MessageStatus(rawValue: status ?? "") ?? .sent
        )
    }
}

@objc(ChatEntity)
class ChatEntity: NSManagedObject {
    @NSManaged var jid: String?
    @NSManaged var name: String?
    @NSManaged var lastMessage: String?
    @NSManaged var lastMessageTime: Date?
    @NSManaged var unreadCount: Int32
    @NSManaged var isGroup: Bool
    @NSManaged var isMuted: Bool
    @NSManaged var isPinned: Bool
    @NSManaged var isArchived: Bool
    @NSManaged var profilePictureURL: String?
    @NSManaged var participantCount: Int32

    func toChat() -> Chat {
        Chat(
            jid: jid ?? "",
            name: name ?? "",
            lastMessage: lastMessage ?? "",
            lastMessageTime: lastMessageTime ?? Date(),
            unreadCount: Int(unreadCount),
            isGroup: isGroup,
            isMuted: isMuted,
            isPinned: isPinned,
            isArchived: isArchived,
            profilePictureURL: profilePictureURL,
            participantCount: participantCount > 0 ? Int(participantCount) : nil
        )
    }
}

/// CoreData persistence controller for efficient message and chat storage
class PersistenceController {
    static let shared = PersistenceController()

    let container: NSPersistentContainer

    // Batch save queue to reduce disk writes
    private var pendingMessages: [Message] = []
    private var pendingChats: [Chat] = []
    private let saveQueue = DispatchQueue(label: "com.whatsapp.persistence", qos: .utility)
    private var saveWorkItem: DispatchWorkItem?

    private init() {
        // Create model programmatically
        let model = Self.createManagedObjectModel()
        container = NSPersistentContainer(name: "WhatsAppModel", managedObjectModel: model)

        container.loadPersistentStores { description, error in
            if let error = error {
                print("CoreData failed to load: \(error.localizedDescription)")
            }
        }

        container.viewContext.mergePolicy = NSMergeByPropertyObjectTrumpMergePolicy
        container.viewContext.automaticallyMergesChangesFromParent = true
    }

    private static func createManagedObjectModel() -> NSManagedObjectModel {
        let model = NSManagedObjectModel()

        // Message Entity
        let messageEntity = NSEntityDescription()
        messageEntity.name = "MessageEntity"
        messageEntity.managedObjectClassName = "MessageEntity"

        let messageAttributes: [(String, NSAttributeType)] = [
            ("id", .stringAttributeType),
            ("chatJID", .stringAttributeType),
            ("senderJID", .stringAttributeType),
            ("senderName", .stringAttributeType),
            ("text", .stringAttributeType),
            ("timestamp", .dateAttributeType),
            ("isFromMe", .booleanAttributeType),
            ("isGroup", .booleanAttributeType),
            ("mediaType", .stringAttributeType),
            ("mediaURL", .stringAttributeType),
            ("mediaCaption", .stringAttributeType),
            ("quotedID", .stringAttributeType),
            ("quotedText", .stringAttributeType),
            ("status", .stringAttributeType)
        ]

        messageEntity.properties = messageAttributes.map { name, type in
            let attr = NSAttributeDescription()
            attr.name = name
            attr.attributeType = type
            attr.isOptional = true
            return attr
        }

        // Add index on id and chatJID for faster lookups
        let messageIdIndex = NSFetchIndexDescription(name: "byId", elements: [
            NSFetchIndexElementDescription(property: messageEntity.propertiesByName["id"]!, collationType: .binary)
        ])
        let messageChatIndex = NSFetchIndexDescription(name: "byChatJID", elements: [
            NSFetchIndexElementDescription(property: messageEntity.propertiesByName["chatJID"]!, collationType: .binary)
        ])
        messageEntity.indexes = [messageIdIndex, messageChatIndex]

        // Chat Entity
        let chatEntity = NSEntityDescription()
        chatEntity.name = "ChatEntity"
        chatEntity.managedObjectClassName = "ChatEntity"

        let chatAttributes: [(String, NSAttributeType)] = [
            ("jid", .stringAttributeType),
            ("name", .stringAttributeType),
            ("lastMessage", .stringAttributeType),
            ("lastMessageTime", .dateAttributeType),
            ("unreadCount", .integer32AttributeType),
            ("isGroup", .booleanAttributeType),
            ("isMuted", .booleanAttributeType),
            ("isPinned", .booleanAttributeType),
            ("isArchived", .booleanAttributeType),
            ("profilePictureURL", .stringAttributeType),
            ("participantCount", .integer32AttributeType)
        ]

        chatEntity.properties = chatAttributes.map { name, type in
            let attr = NSAttributeDescription()
            attr.name = name
            attr.attributeType = type
            attr.isOptional = true
            return attr
        }

        // Add index on jid for faster lookups
        let chatJidIndex = NSFetchIndexDescription(name: "byJid", elements: [
            NSFetchIndexElementDescription(property: chatEntity.propertiesByName["jid"]!, collationType: .binary)
        ])
        chatEntity.indexes = [chatJidIndex]

        model.entities = [messageEntity, chatEntity]
        return model
    }

    var viewContext: NSManagedObjectContext {
        container.viewContext
    }

    func newBackgroundContext() -> NSManagedObjectContext {
        let context = container.newBackgroundContext()
        context.mergePolicy = NSMergeByPropertyObjectTrumpMergePolicy
        return context
    }

    // MARK: - Batched Save (reduces CPU usage)

    /// Queue a message for batch saving (debounced)
    func queueMessage(_ message: Message) {
        saveQueue.async { [weak self] in
            self?.pendingMessages.append(message)
            self?.scheduleBatchSave()
        }
    }

    /// Queue a chat update for batch saving (debounced)
    func queueChatUpdate(_ chat: Chat) {
        saveQueue.async { [weak self] in
            // Replace existing or append
            if let index = self?.pendingChats.firstIndex(where: { $0.jid == chat.jid }) {
                self?.pendingChats[index] = chat
            } else {
                self?.pendingChats.append(chat)
            }
            self?.scheduleBatchSave()
        }
    }

    /// Queue a chat update from message
    func queueChatUpdateForMessage(_ message: Message) {
        saveQueue.async { [weak self] in
            self?.updateChatForMessageInternal(message)
            self?.scheduleBatchSave()
        }
    }

    private func scheduleBatchSave() {
        saveWorkItem?.cancel()
        let workItem = DispatchWorkItem { [weak self] in
            self?.performBatchSave()
        }
        saveWorkItem = workItem
        // Debounce: wait 0.5 seconds before saving
        saveQueue.asyncAfter(deadline: .now() + 0.5, execute: workItem)
    }

    private func performBatchSave() {
        let messagesToSave = pendingMessages
        let chatsToSave = pendingChats
        pendingMessages = []
        pendingChats = []

        guard !messagesToSave.isEmpty || !chatsToSave.isEmpty else { return }

        let context = newBackgroundContext()
        context.perform {
            // Save messages
            for message in messagesToSave {
                let fetchRequest = NSFetchRequest<MessageEntity>(entityName: "MessageEntity")
                fetchRequest.predicate = NSPredicate(format: "id == %@", message.id)
                fetchRequest.fetchLimit = 1

                do {
                    let existing = try context.fetch(fetchRequest)
                    if existing.isEmpty {
                        let entity = MessageEntity(entity: context.persistentStoreCoordinator!.managedObjectModel.entitiesByName["MessageEntity"]!, insertInto: context)
                        entity.id = message.id
                        entity.chatJID = message.chatJID
                        entity.senderJID = message.senderJID
                        entity.senderName = message.senderName
                        entity.text = message.text
                        entity.timestamp = message.timestamp
                        entity.isFromMe = message.isFromMe
                        entity.isGroup = message.isGroup
                        entity.mediaType = message.mediaType.rawValue
                        entity.mediaURL = message.mediaURL
                        entity.mediaCaption = message.mediaCaption
                        entity.quotedID = message.quotedID
                        entity.quotedText = message.quotedText
                        entity.status = message.status.rawValue
                    }
                } catch {
                    print("Failed to check existing message: \(error)")
                }
            }

            // Save chats
            for chat in chatsToSave {
                let fetchRequest = NSFetchRequest<ChatEntity>(entityName: "ChatEntity")
                fetchRequest.predicate = NSPredicate(format: "jid == %@", chat.jid)
                fetchRequest.fetchLimit = 1

                do {
                    let existing = try context.fetch(fetchRequest)
                    let entity: ChatEntity
                    if let existingChat = existing.first {
                        entity = existingChat
                    } else {
                        entity = ChatEntity(entity: context.persistentStoreCoordinator!.managedObjectModel.entitiesByName["ChatEntity"]!, insertInto: context)
                    }

                    entity.jid = chat.jid
                    entity.name = chat.name
                    entity.lastMessage = chat.lastMessage
                    entity.lastMessageTime = chat.lastMessageTime
                    entity.unreadCount = Int32(chat.unreadCount)
                    entity.isGroup = chat.isGroup
                    entity.isMuted = chat.isMuted
                    entity.isPinned = chat.isPinned
                    entity.isArchived = chat.isArchived
                    entity.profilePictureURL = chat.profilePictureURL
                    entity.participantCount = Int32(chat.participantCount ?? 0)
                } catch {
                    print("Failed to check existing chat: \(error)")
                }
            }

            do {
                try context.save()
            } catch {
                print("Failed batch save: \(error)")
            }
        }
    }

    private func updateChatForMessageInternal(_ message: Message) {
        // Find existing chat in pending or create new
        if let index = pendingChats.firstIndex(where: { $0.jid == message.chatJID }) {
            var chat = pendingChats[index]
            if message.timestamp > chat.lastMessageTime {
                chat.lastMessage = message.displayText
                chat.lastMessageTime = message.timestamp
            }
            if !message.isFromMe {
                chat.unreadCount += 1
            }
            pendingChats[index] = chat
        } else {
            // Need to fetch from DB or create new
            let newChat = Chat(
                jid: message.chatJID,
                name: message.senderName.isEmpty ? message.senderJID : message.senderName,
                lastMessage: message.displayText,
                lastMessageTime: message.timestamp,
                unreadCount: message.isFromMe ? 0 : 1,
                isGroup: message.isGroup,
                isMuted: false,
                isPinned: false,
                isArchived: false
            )
            pendingChats.append(newChat)
        }
    }

    // MARK: - Fetch Operations

    func fetchMessages(for chatJID: String) -> [Message] {
        let fetchRequest = NSFetchRequest<MessageEntity>(entityName: "MessageEntity")
        fetchRequest.predicate = NSPredicate(format: "chatJID == %@", chatJID)
        fetchRequest.sortDescriptors = [NSSortDescriptor(key: "timestamp", ascending: true)]

        do {
            let entities = try viewContext.fetch(fetchRequest)
            return entities.map { $0.toMessage() }
        } catch {
            print("Failed to fetch messages: \(error)")
            return []
        }
    }

    func fetchAllMessages() -> [String: [Message]] {
        let fetchRequest = NSFetchRequest<MessageEntity>(entityName: "MessageEntity")
        fetchRequest.sortDescriptors = [NSSortDescriptor(key: "timestamp", ascending: true)]

        do {
            let entities = try viewContext.fetch(fetchRequest)
            var result: [String: [Message]] = [:]
            for entity in entities {
                let message = entity.toMessage()
                if result[message.chatJID] == nil {
                    result[message.chatJID] = []
                }
                result[message.chatJID]?.append(message)
            }
            return result
        } catch {
            print("Failed to fetch all messages: \(error)")
            return [:]
        }
    }

    func fetchAllChats() -> [Chat] {
        let fetchRequest = NSFetchRequest<ChatEntity>(entityName: "ChatEntity")
        fetchRequest.sortDescriptors = [
            NSSortDescriptor(key: "isPinned", ascending: false),
            NSSortDescriptor(key: "lastMessageTime", ascending: false)
        ]

        do {
            let entities = try viewContext.fetch(fetchRequest)
            return entities.map { $0.toChat() }
        } catch {
            print("Failed to fetch chats: \(error)")
            return []
        }
    }

    // MARK: - Clear Data

    func clearAllData() {
        saveQueue.async { [weak self] in
            self?.pendingMessages = []
            self?.pendingChats = []
        }

        let context = newBackgroundContext()
        context.perform {
            let messageRequest = NSFetchRequest<NSFetchRequestResult>(entityName: "MessageEntity")
            let messageDelete = NSBatchDeleteRequest(fetchRequest: messageRequest)

            let chatRequest = NSFetchRequest<NSFetchRequestResult>(entityName: "ChatEntity")
            let chatDelete = NSBatchDeleteRequest(fetchRequest: chatRequest)

            do {
                try context.execute(messageDelete)
                try context.execute(chatDelete)
                try context.save()
            } catch {
                print("Failed to clear data: \(error)")
            }
        }
    }

    /// Force save any pending data immediately
    func flushPendingData() {
        saveQueue.sync {
            saveWorkItem?.cancel()
            performBatchSave()
        }
    }
}
