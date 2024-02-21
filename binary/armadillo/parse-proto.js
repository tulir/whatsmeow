///////////////////
// JS EVALUATION //
///////////////////

const protos = []
const modules = {
	"$InternalEnum": {
		exports: {
			exports: function (data) {
				data.__enum__ = true
				return data
			}
		}
	},
}

function requireModule(name) {
	if (!modules[name]) {
		throw new Error(`Unknown requirement ${name}`)
	}
	return modules[name].exports
}

function requireDefault(name) {
	return requireModule(name).exports
}

function ignoreModule(name) {
	if (name === "WAProtoConst") {
		return false
	} else if (!name.endsWith(".pb")) {
		// Ignore any non-protobuf modules, except WAProtoConst above
		return true
	} else if (name.startsWith("MAWArmadillo") && (name.endsWith("TableSchema.pb") || name.endsWith("TablesSchema.pb"))) {
		// Ignore internal table schemas
		return true
	} else if (name === "WASignalLocalStorageProtocol.pb" || name === "WASignalWhisperTextProtocol.pb") {
		// Ignore standard signal protocol stuff
		return true
	} else {
		return false
	}
}

function defineModule(name, dependencies, callback, unknownIntOrNull) {
	if (ignoreModule(name)) {
		return
	}
	const exports = {}
	if (dependencies.length > 0) {
		callback(null, requireDefault, null, requireModule, null, null, exports)
	} else {
		callback(null, requireDefault, null, requireModule, exports, exports)
	}
	modules[name] = {exports, dependencies}
}

global.self = global
global.__d = defineModule

require("./e2ee.js")

function dereference(obj, module, currentPath, next, ...remainder) {
	if (!next) {
		return obj
	}
	if (!obj.messages[next]) {
		obj.messages[next] = {messages: {}, enums: {}, __module__: module, __path__: currentPath, __name__: next}
	}
	return dereference(obj.messages[next], module, currentPath.concat([next]), ...remainder)
}

function dereferenceSnake(obj, currentPath, path) {
	let next = path[0]
	path = path.slice(1)
	while (!obj.messages[next]) {
		if (path.length === 0) {
			return [obj, currentPath, next]
		}
		next += path[0]
		path = path.slice(1)
	}
	return dereferenceSnake(obj.messages[next], currentPath.concat([next]), path)
}

function renameModule(name) {
	return name.replace(".pb", "")
}

function renameDependencies(dependencies) {
	return dependencies
		.filter(name => name.endsWith(".pb"))
		.map(renameModule)
		.map(name => name === "WAProtocol" ? "WACommon" : name)
}

function renameType(protoName, fieldName, field) {
	return fieldName
}

for (const [name, module] of Object.entries(modules)) {
	if (!name.endsWith(".pb")) {
		continue
	} else if (!module.exports) {
		console.warn(name, "has no exports")
		continue
	}
	// Slightly hacky way to get rid of WAProtocol.pb and just use the MessageKey in WACommon
	if (name === "WAProtocol.pb") {
		if (Object.entries(module.exports).length > 1) {
			console.warn("WAProtocol.pb has more than one export")
		}
		module.exports["MessageKeySpec"].__name__ = "MessageKey"
		module.exports["MessageKeySpec"].__module__ = "WACommon"
		module.exports["MessageKeySpec"].__path__ = []
		continue
	}
	const proto = {
		__protobuf__: true,
		messages: {},
		enums: {},
		__name__: renameModule(name),
		dependencies: renameDependencies(module.dependencies),
	}
	const upperSnakeEnums = []
	for (const [name, field] of Object.entries(module.exports)) {
		const namePath = name.replace(/Spec$/, "").split("$")
		field.__name__ = renameType(proto.__name__, namePath[namePath.length - 1], field)
		namePath[namePath.length - 1] = field.__name__
		field.__path__ = namePath.slice(0, -1)
		field.__module__ = proto.__name__
		if (field.internalSpec) {
			dereference(proto, proto.__name__, [], ...namePath).message = field.internalSpec
		} else if (namePath.length === 1 && name.toUpperCase() === name) {
			upperSnakeEnums.push(field)
		} else {
			dereference(proto, proto.__name__, [], ...namePath.slice(0, -1)).enums[field.__name__] = field
		}
	}
	// Some enums have uppercase names, instead of capital case with $ separators.
	// For those, we need to find the right nesting location.
	for (const field of upperSnakeEnums) {
		field.__enum__ = true
		const [obj, path, name] = dereferenceSnake(proto, [], field.__name__.split("_").map(part => part[0] + part.slice(1).toLowerCase()))
		field.__path__ = path
		field.__name__ = name
		field.__module__ = proto.__name__
		obj.enums[name] = field
	}
	protos.push(proto)
}

////////////////////////////////
// PROTOBUF SCHEMA GENERATION //
////////////////////////////////

function indent(lines, indent = "\t") {
	return lines.map(line => line ? `${indent}${line}` : "")
}

function flattenWithBlankLines(...items) {
	return items
		.flatMap(item => item.length > 0 ? [item, [""]] : [])
		.slice(0, -1)
		.flatMap(item => item)
}

function protoifyChildren(container) {
	return flattenWithBlankLines(
		...Object.values(container.enums).map(protoifyEnum),
		...Object.values(container.messages).map(protoifyMessage),
	)
}

function protoifyEnum(enumDef) {
	const values = []
	const names = Object.fromEntries(Object.entries(enumDef).map(([name, value]) => [value, name]))
	if (!names["0"]) {
		if (names["-1"]) {
			enumDef[names["-1"]] = 0
		} else {
			// TODO add snake case
			values.push(`${enumDef.__name__.toUpperCase()}_UNKNOWN = 0;`)
		}
	}
	for (const [name, value] of Object.entries(enumDef)) {
		if (name.startsWith("__") && name.endsWith("__")) {
			continue
		}
		values.push(`${name} = ${value};`)
	}
	return [`enum ${enumDef.__name__} ` + "{", ...indent(values), "}"]
}

const {TYPES, TYPE_MASK, FLAGS} = requireModule("WAProtoConst")

function fieldTypeName(typeID, typeRef, parentModule, parentPath) {
	switch (typeID) {
		case TYPES.INT32:
			return "int32"
		case TYPES.INT64:
			return "int64"
		case TYPES.UINT32:
			return "uint32"
		case TYPES.UINT64:
			return "uint64"
		case TYPES.SINT32:
			return "sint32"
		case TYPES.SINT64:
			return "sint64"
		case TYPES.BOOL:
			return "bool"
		case TYPES.ENUM:
		case TYPES.MESSAGE:
			let pathStartIndex = 0
			for (let i = 0; i < parentPath.length && i < typeRef.__path__.length; i++) {
				if (typeRef.__path__[i] === parentPath[i]) {
					pathStartIndex++
				} else {
					break
				}
			}
			const namePath = []
			if (typeRef.__module__ !== parentModule) {
				namePath.push(typeRef.__module__)
				pathStartIndex = 0
			}
			namePath.push(...typeRef.__path__.slice(pathStartIndex))
			namePath.push(typeRef.__name__)
			return namePath.join(".")
		case TYPES.FIXED64:
			return "fixed64"
		case TYPES.SFIXED64:
			return "sfixed64"
		case TYPES.DOUBLE:
			return "double"
		case TYPES.STRING:
			return "string"
		case TYPES.BYTES:
			return "bytes"
		case TYPES.FIXED32:
			return "fixed32"
		case TYPES.SFIXED32:
			return "sfixed32"
		case TYPES.FLOAT:
			return "float"
	}
}

const staticRenames = {
	id: "ID",
	jid: "JID",
	encIv: "encIV",
	iv: "IV",
	ptt: "PTT",
	hmac: "HMAC",
	url: "URL",
	fbid: "FBID",
	jpegThumbnail: "JPEGThumbnail",
	dsm: "DSM",
}

function fixFieldName(name) {
	if (name === "id") {
		return "ID"
	} else if (name === "encIv") {
		return "encIV"
	}
	return staticRenames[name] ?? name
		.replace(/Id([A-Zs]|$)/, "ID$1")
		.replace("Jid", "JID")
		.replace(/Ms([A-Z]|$)/, "MS$1")
		.replace(/Ts([A-Z]|$)/, "TS$1")
		.replace(/Mac([A-Z]|$)/, "MAC$1")
		.replace("Url", "URL")
		.replace("Cdn", "CDN")
		.replace("Json", "JSON")
		.replace("Jpeg", "JPEG")
		.replace("Sha256", "SHA256")
}

function protoifyField(name, [index, flags, typeRef], parentModule, parentPath) {
	const preflags = []
	const postflags = [""]
	if ((flags & FLAGS.REPEATED) !== 0) {
		preflags.push("repeated")
	}
	// if ((flags & FLAGS.REQUIRED) === 0) {
	// 	preflags.push("optional")
	// } else {
	// 	preflags.push("required")
	// }
	preflags.push(fieldTypeName(flags & TYPE_MASK, typeRef, parentModule, parentPath))
	if ((flags & FLAGS.PACKED) !== 0) {
		postflags.push(`[packed=true]`)
	}
	return `${preflags.join(" ")} ${fixFieldName(name)} = ${index}${postflags.join(" ")};`
}

function protoifyFields(fields, parentModule, parentPath) {
	return Object.entries(fields).map(([name, definition]) => protoifyField(name, definition, parentModule, parentPath))
}

function protoifyMessage(message) {
	const sections = [protoifyChildren(message)]
	const spec = message.message
	const fullMessagePath = message.__path__.concat([message.__name__])
	for (const [name, fieldNames] of Object.entries(spec.__oneofs__ ?? {})) {
		const fields = Object.fromEntries(fieldNames.map(fieldName => {
			const def = spec[fieldName]
			delete spec[fieldName]
			return [fieldName, def]
		}))
		sections.push([`oneof ${name} ` + "{", ...indent(protoifyFields(fields, message.__module__, fullMessagePath)), "}"])
	}
	if (spec.__reserved__) {
		console.warn("Found reserved keys:", message.__name__, spec.__reserved__)
	}
	delete spec.__oneofs__
	delete spec.__reserved__
	sections.push(protoifyFields(spec, message.__module__, fullMessagePath))
	return [`message ${message.__name__} ` + "{", ...indent(flattenWithBlankLines(...sections)), "}"]
}

function goPackageName(name) {
	return name.replace(/^WA/, "wa")
}

function protoifyModule(module) {
	const output = []
	output.push(`syntax = "proto3";`)
	output.push(`package ${module.__name__};`)
	output.push(`option go_package = "go.mau.fi/whatsmeow/binary/armadillo/${goPackageName(module.__name__)}";`)
	output.push("")
	if (module.dependencies.length > 0) {
		for (const dependency of module.dependencies) {
			output.push(`import "${goPackageName(dependency)}/${dependency}.proto";`)
		}
		output.push("")
	}
	const children = protoifyChildren(module)
	children.push("")
	return output.concat(children)
}

const fs = require("fs")

for (const proto of protos) {
	fs.mkdirSync(goPackageName(proto.__name__), {recursive: true})
	fs.writeFileSync(`${goPackageName(proto.__name__)}/${proto.__name__}.proto`, protoifyModule(proto).join("\n"))
}
