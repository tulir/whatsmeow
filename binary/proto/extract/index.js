const request = require("request-promise-native")
const acorn = require("acorn")
const walk = require("acorn-walk")
const fs = require("fs/promises")

const addPrefix = (lines, prefix) => lines.map(line => prefix + line)

async function findAppModules(mods) {
    const ua = {
        headers: {
            "User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/115.0",
            "Sec-Fetch-Dest": "script",
            "Sec-Fetch-Mode": "no-cors",
            "Sec-Fetch-Site": "same-origin",
            "Referer": "https://web.whatsapp.com/",
            "Accept": "*/*", /**/
            "Accept-Language": "en-US,en;q=0.5",
        }
    }
    const baseURL = "https://web.whatsapp.com"
    const index = await request.get(baseURL, ua)
    const appID = index.match(/src="\/app.([0-9a-z]{10,}).js"/)[1]
    const appURL = baseURL + "/app." + appID + ".js"
    console.error("Found app.js URL:", appURL)
    const qrData = await request.get(appURL, ua)
    const waVersion = qrData.match(/VERSION_BASE="(\d\.\d+\.\d+)"/)[1]
    console.log("Current version:", waVersion)
    // This one list of types is so long that it's split into two JavaScript declarations.
    // The module finder below can't handle it, so just patch it manually here.
    const patchedQrData = qrData.replace("t.ActionLinkSpec=void 0,t.TemplateButtonSpec", "t.ActionLinkSpec=t.TemplateButtonSpec")
    //const patchedQrData = qrData.replace("Spec=void 0,t.", "Spec=t.")
    const qrModules = acorn.parse(patchedQrData).body[0].expression.arguments[0].elements[1].properties
    return qrModules.filter(m => mods.includes(m.key.value))
}

(async () => {
    // The module IDs that contain protobuf types
    const wantedModules = [
        962559, // ADVSignedKeyIndexList, ADVSignedDeviceIdentity, ADVSignedDeviceIdentityHMAC, ADVKeyIndexList, ADVDeviceIdentity
        113259, // DeviceProps
        533494, // Message, ..., RequestPaymentMessage, Reaction, QuickReplyButton, ..., ButtonsResponseMessage, ActionLink, ...
        199931, // EphemeralSetting
        60370, // WallpaperSettings, Pushname, MediaVisibility, HistorySync, ..., GroupParticipant, ...
        //412744, // PollEncValue, MsgOpaqueData, MsgRowOpaqueData
        229479, // ServerErrorReceipt, MediaRetryNotification, MediaRetryNotificationResult
        933734, // MessageKey
        557871, // Duplicate of MessageKey
        679905, // SyncdVersion, SyncdValue, ..., SyncdPatch, SyncdMutation, ..., ExitCode
        623420, // SyncActionValue, ..., UnarchiveChatsSetting, SyncActionData, StarAction, ...
        //527796, // Duplicate of 623420, but without CallLogRecord
        759089, // VerifiedNameCertificate, LocalizedName, ..., BizIdentityInfo, BizAccountLinkInfo, ...
        614806, // HandshakeMessage, ..., ClientPayload, ..., AppVersion, UserAgent, WebdPayload ...
        968923, // Reaction, UserReceipt, ..., PhotoChange, ..., WebFeatures, ..., WebMessageInfoStatus, ...
        623641, // NoiseCertificate, CertChain
        //867311, // ChatRowOpaqueData, ...
        //2336, // SignalMessage, ...
        //984661, // SessionStructure, ...
        853721, // QP
    ]
    const unspecName = name => name.endsWith("Spec") ? name.slice(0, -4) : name
    const unnestName = name => name.replace("Message$", "").replace("SyncActionValue$", "") // Don't nest messages into Message, that's too much nesting
    const rename = name => unnestName(unspecName(name))
    // The constructor IDs that can be used for enum types
    const enumConstructorIDs = [76672, 654302]

    const unsortedModules = await findAppModules(wantedModules)
    if (unsortedModules.length !== wantedModules.length) {
        console.error("did not find all wanted modules")
        return
    }
    // Sort modules so that whatsapp module id changes don't change the order in the output protobuf schema
    const modules = []
    for (const mod of wantedModules) {
        modules.push(unsortedModules.find(node => node.key.value === mod))
    }

    // find aliases of cross references between the wanted modules
    let modulesInfo = {}
    modules.forEach(({key, value}) => {
        const requiringParam = value.params[2].name
        modulesInfo[key.value] = {crossRefs: []}
        walk.simple(value, {
            VariableDeclarator(node) {
                if (node.init && node.init.type === "CallExpression" && node.init.callee.name === requiringParam && node.init.arguments.length === 1 && wantedModules.indexOf(node.init.arguments[0].value) !== -1) {
                    modulesInfo[key.value].crossRefs.push({alias: node.id.name, module: node.init.arguments[0].value})
                }
            }
        })
    })

    // find all identifiers and, for enums, their array of values
    for (const mod of modules) {
        const modInfo = modulesInfo[mod.key.value]

        // all identifiers will be initialized to "void 0" (i.e. "undefined") at the start, so capture them here
        walk.ancestor(mod, {
            UnaryExpression(node, anc) {
                if (!modInfo.identifiers && node.operator === "void") {
                    const assignments = []
                    let i = 1
                    anc.reverse()
                    while (anc[i].type === "AssignmentExpression") {
                        assignments.push(anc[i++].left)
                    }
                    const makeBlankIdent = a => {
                        const key = rename(a.property.name)
                        const value = {name: key}
                        return [key, value]
                    }
                    modInfo.identifiers = Object.fromEntries(assignments.map(makeBlankIdent).reverse())
                }
            }
        })
        const enumAliases = {}
        // enums are defined directly, and both enums and messages get a one-letter alias
        walk.simple(mod, {
            AssignmentExpression(node) {
                if (node.left.type === "MemberExpression" && modInfo.identifiers[rename(node.left.property.name)]) {
                    const ident = modInfo.identifiers[rename(node.left.property.name)]
                    ident.alias = node.right.name
                    ident.enumValues = enumAliases[ident.alias]
                }
            },
            VariableDeclarator(node) {
                if (node.init && node.init.type === "CallExpression" && enumConstructorIDs.includes(node.init.callee?.arguments?.[0]?.value) && node.init.arguments.length === 1 && node.init.arguments[0].type === "ObjectExpression") {
                    enumAliases[node.id.name] = node.init.arguments[0].properties.map(p => ({
                        name: p.key.name,
                        id: p.value.value
                    }))
                }
            }
        })
    }

    // find the contents for all protobuf messages
    for (const mod of modules) {
        const modInfo = modulesInfo[mod.key.value]

        // message specifications are stored in a "internalSpec" attribute of the respective identifier alias
        walk.simple(mod, {
            AssignmentExpression(node) {
                if (node.left.type === "MemberExpression" && node.left.property.name === "internalSpec" && node.right.type === "ObjectExpression") {
                    const targetIdent = Object.values(modInfo.identifiers).find(v => v.alias === node.left.object.name)
                    if (!targetIdent) {
                        console.warn(`found message specification for unknown identifier alias: ${node.left.object.name}`)
                        return
                    }

                    // partition spec properties by normal members and constraints (like "__oneofs__") which will be processed afterwards
                    const constraints = []
                    let members = []
                    for (const p of node.right.properties) {
                        p.key.name = p.key.type === "Identifier" ? p.key.name : p.key.value
                        ;(p.key.name.substr(0, 2) === "__" ? constraints : members).push(p)
                    }

                    members = members.map(({key: {name}, value: {elements}}) => {
                        let type
                        const flags = []
                        const unwrapBinaryOr = n => (n.type === "BinaryExpression" && n.operator === "|") ? [].concat(unwrapBinaryOr(n.left), unwrapBinaryOr(n.right)) : [n]

                        // find type and flags
                        unwrapBinaryOr(elements[1]).forEach(m => {
                            if (m.type === "MemberExpression" && m.object.type === "MemberExpression") {
                                if (m.object.property.name === "TYPES")
                                    type = m.property.name.toLowerCase()
                                else if (m.object.property.name === "FLAGS")
                                    flags.push(m.property.name.toLowerCase())
                            }
                        })

                        // determine cross reference name from alias if this member has type "message" or "enum"
                        if (type === "message" || type === "enum") {
                            const currLoc = ` from member '${name}' of message '${targetIdent.name}'`
                            if (elements[2].type === "Identifier") {
                                type = Object.values(modInfo.identifiers).find(v => v.alias === elements[2].name)?.name
                                if (!type) {
                                    console.warn(`unable to find reference of alias '${elements[2].name}'` + currLoc)
                                }
                            } else if (elements[2].type === "MemberExpression") {
                                const crossRef = modInfo.crossRefs.find(r => r.alias === elements[2].object.name)
                                if (crossRef && modulesInfo[crossRef.module].identifiers[rename(elements[2].property.name)]) {
                                    type = rename(elements[2].property.name)
                                } else {
                                    console.warn(`unable to find reference of alias to other module '${elements[2].object.name}' or to message ${elements[2].property.name} of this module` + currLoc)
                                }
                            }
                        }

                        return {name, id: elements[0].value, type, flags}
                    })

                    // resolve constraints for members
                    constraints.forEach(c => {
                        if (c.key.name === "__oneofs__" && c.value.type === "ObjectExpression") {
                            const newOneOfs = c.value.properties.map(p => ({
                                name: p.key.name,
                                type: "__oneof__",
                                members: p.value.elements.map(e => {
                                    const idx = members.findIndex(m => m.name === e.value)
                                    const member = members[idx]
                                    members.splice(idx, 1)
                                    return member
                                })
                            }))
                            members.push(...newOneOfs)
                        }
                    })

                    targetIdent.members = members
                    targetIdent.children = []
                }
            }
        })
    }

    const findNested = (items, path) => {
        if (path.length === 0) {
            return items
        }
        const item = items.find(v => (v.unnestedName ?? v.name) === path[0])
        if (path.length === 1) {
            return item
        }
        return findNested(item.children, path.slice(1))
    }


    for (const mod of modules) {
        let hasMore = true
        let loops = 0
        const idents = modulesInfo[mod.key.value].identifiers
        while (hasMore && loops < 5) {
            hasMore = false
            loops++
            for (const ident of Object.values(idents)) {
                if (!ident.name.includes("$")) {
                    continue
                }
                const parts = ident.name.split("$")
                const parent = findNested(Object.values(idents), parts.slice(0, -1))
                if (!parent) {
                    hasMore = true
                    continue
                }
                parent.children.push(ident)
                delete idents[ident.name]
                ident.unnestedName = parts[parts.length-1]
            }
        }
    }

    const addedMessages = new Set()
    let decodedProto = [
        'syntax = "proto2";',
        "package proto;",
        ""
    ]
    const sharesParent = (path1, path2) => {
        for (let i = 0; i < path1.length - 1 && i < path2.length - 1; i++) {
            if (path1[i] != path2[i]) {
                return false
            }
        }
        return true
    }
    const spaceIndent = " ".repeat(4)
    for (const mod of modules) {
        const modInfo = modulesInfo[mod.key.value]

        // enum stringifying function
        const stringifyEnum = (ident, overrideName = null) =>
            [].concat(
                [`enum ${overrideName ?? ident.unnestedName ?? ident.name} {`],
                addPrefix(ident.enumValues.map(v => `${v.name} = ${v.id};`), spaceIndent),
                ["}"]
            )

        // message specification member stringifying function
        const stringifyMessageSpecMember = (info, path, completeFlags = true) => {
            if (info.type === "__oneof__") {
                return [].concat(
                    [`oneof ${info.name} {`],
                    addPrefix([].concat(...info.members.map(m => stringifyMessageSpecMember(m, path, false))), spaceIndent),
                    ["}"]
                )
            } else {
                if (info.flags.includes("packed")) {
                    info.flags.splice(info.flags.indexOf("packed"))
                    info.packed = " [packed=true]"
                }
                if (completeFlags && info.flags.length === 0) {
                    info.flags.push("optional")
                }
                const ret = info.enumValues ? stringifyEnum(info, info.type) : []
                const typeParts = info.type.split("$")
                let unnestedType = typeParts[typeParts.length-1]
                if (!sharesParent(typeParts, path.split("$"))) {
                    unnestedType = typeParts.join(".")
                }
                ret.push(`${info.flags.join(" ") + (info.flags.length === 0 ? "" : " ")}${unnestedType} ${info.name} = ${info.id}${info.packed || ""};`)
                return ret
            }
        }

        // message specification stringifying function
        const stringifyMessageSpec = (ident) => {
            let result = []
            result.push(
                `message ${ident.unnestedName ?? ident.name} {`,
                ...addPrefix([].concat(...ident.children.map(m => stringifyEntity(m))), spaceIndent),
                ...addPrefix([].concat(...ident.members.map(m => stringifyMessageSpecMember(m, ident.name))), spaceIndent),
                "}",
            )
            if (addedMessages.has(ident.name)) {
                result = addPrefix(result, "//")
                result.unshift("// Duplicate type omitted")
            } else {
                addedMessages.add(ident.name)
            }
            result.push("")
            return result
        }

        const stringifyEntity = v => {
            if (v.members) {
                return stringifyMessageSpec(v)
            } else if (v.enumValues) {
                return stringifyEnum(v)
            } else {
                console.error(v)
                return "// Unknown entity"
            }
        }

        decodedProto = decodedProto.concat(...Object.values(modInfo.identifiers).map(stringifyEntity))
    }
    const decodedProtoStr = decodedProto.join("\n") + "\n"
    await fs.writeFile("../def.proto", decodedProtoStr)
    console.log("Extracted protobuf schema to ../def.proto")
})()
