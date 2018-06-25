// type format (typeInfo) grammar
// typeInfo describe the types with encoding
// type: bool | int | uint | address | bytes | enum | string | fix_array | dyn_array | struct
// fix_array: fix_array_start [0-9]+ type
// dyn_array: dyn_array_start type
// struct: struct_start type+ struct_end

// TODO: bytes
package arg_parser

import "math/big"
import "bytes"
import "fmt"
import "errors"

import "github.com/ethereum/go-ethereum/core/vm/eni/typecodes"

func Parse(typeInfo []byte, data []byte) (ret string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprint("Argument Parser Error: ", r))
		}
	}()
	var json bytes.Buffer
	parseEntryPoint(typeInfo, data, &json)
	return json.String(), err
}

func parseEntryPoint(typeInfo []byte, data []byte, json *bytes.Buffer) {
	json.WriteString("[")
	for i := 0; 0 < len(typeInfo); i++ {
		if 0 < i {
			json.WriteString(",")
		}
		typeInfo, data = parseType(typeInfo, data, json)
	}
	json.WriteString("]")
}

// assuming that data are packed
func parseType(typeInfo []byte, data []byte, json *bytes.Buffer) ([]byte, []byte) {
	t := typeInfo[0]
	if typecodes.ComplexType[t] {
		if t == typecodes.FIX_ARRAY_START {
			typeInfo, data = parseFixArray(typeInfo, data, json)
		} else if t == typecodes.DYN_ARRAY_START {
			typeInfo, data = parseDynArray(typeInfo, data, json)
		} else if t == typecodes.STRUCT_START {
			typeInfo, data = parseStruct(typeInfo, data, json)
		} else if t == typecodes.STRING {
			typeInfo, data = parseString(typeInfo, data, json)
		}
	} else { // value type
		typeInfo, data = parseValue(typeInfo, data, json)
	}
	return typeInfo, data
}

func parseString(typeInfo []byte, data []byte, json *bytes.Buffer) ([]byte, []byte) {
	typeInfo = typeInfo[1:] // string
	leng := new(big.Int).SetBytes(data[:32]).Int64()
	data = data[32:]

	var buffer bytes.Buffer
	for i := int64(0); i < leng; i++ {
		if data[i] == '\\' || data[i] == '"' {
			buffer.WriteByte('\\')
			buffer.WriteByte(data[i])
		} else if data[i] < 0x20 { // control characters
			buffer.WriteString(fmt.Sprintf("\\u%04X", data[i]))
		} else {
			buffer.WriteByte(data[i])
		}
	}
	json.WriteString("\"")
	json.WriteString(buffer.String())
	json.WriteString("\"")
	data = data[leng:]
	if leng%32 > 0 {
		data = data[32-leng%32:]
	}
	return typeInfo, data
}

// parsing int32 not finished
func parseFixArray(typeInfo []byte, data []byte, json *bytes.Buffer) ([]byte, []byte) {
	typeInfo = typeInfo[1:] // fix_array_start
	json.WriteString("[")
	leng := new(big.Int).SetBytes(typeInfo[:32]).Int64()
	typeInfo = typeInfo[32:]

	for i := int64(0); i < leng; i++ {
		if i == leng-1 {
			typeInfo, data = parseType(typeInfo, data, json)
		} else {
			json.WriteString(", ")
			_, data = parseType(typeInfo, data, json)
		}
	}

	json.WriteString("]")
	return typeInfo, data
}

// dynamic array
func parseDynArray(typeInfo []byte, data []byte, json *bytes.Buffer) ([]byte, []byte) {
	panic(fmt.Sprintf("dynamic array not implemented yet!"))
}

func parseStruct(typeInfo []byte, data []byte, json *bytes.Buffer) ([]byte, []byte) {
	typeInfo = typeInfo[1:] // struct_start
	json.WriteString("[")
	for i := 0; 0 < len(typeInfo); i++ {
		t := typeInfo[0]
		if 0 < i {
			json.WriteString(", ")
		}
		if t != typecodes.STRUCT_END {
			typeInfo, data = parseType(typeInfo, data, json)
		}
	}
	if typeInfo[0] != typecodes.STRUCT_START {
		panic("encoding error - expected struct_end token")
	}
	typeInfo = typeInfo[1:] // struct_end
	json.WriteString("]")
	return typeInfo, data
}

// bool, int
func parseValue(typeInfo []byte, data []byte, json *bytes.Buffer) ([]byte, []byte) {
	t := typeInfo[0]
	if t == typecodes.BOOL {
		var boolVal bool
		for i := 0; i < 32; i++ {
			if data[i] != 0 {
				boolVal = true
			}
		}
		json.WriteString(fmt.Sprint(boolVal))
	} else if typecodes.IsSint(t) { // signed integer
		n := new(big.Int)
		var b [32]byte
		copy(b[:], data[:32])
		if b[0] >= 128 { // negative value, two's complement
			n.SetBytes(b[:])
			n = n.Sub(n, big.NewInt(int64(1)))
			copy(b[:], n.Bytes())
			for i := 0; i < 32; i++ {
				b[i] ^= 255
			}
			n.SetBytes(b[:])
			n = n.Mul(n, big.NewInt(int64(-1)))
			json.WriteString(n.String())
		} else { // positive value
			n.SetBytes(b[:])
			json.WriteString(n.String())
		}

	} else if typecodes.IsUint(t) { // unsigned integer
		n := new(big.Int)
		n.SetBytes(data[:32]) // big endian
		json.WriteString(n.String())
	} else {
		panic(fmt.Sprintf("encoding error - unknown or not implemented type: %d", t))
	}
	typeInfo = typeInfo[1:]
	data = data[32:]
	return typeInfo, data
}