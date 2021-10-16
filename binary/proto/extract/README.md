# proto/extract
This is an updated version of the [protobuf extractor from sigalor/whatsapp-web-reveng](https://github.com/sigalor/whatsapp-web-reveng/tree/master/doc/spec/protobuf-extractor).

## Usage
1. Install dependencies with `yarn` (or `npm install`)
2. `node index.js | sed 's/Spec//g' > ../def.proto`
3. Apply manual fixes (TODO: automate this?)
