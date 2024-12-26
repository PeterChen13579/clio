package shamap

// TODO: the paths here are hardcoded, we need to find a way to make them dynamic
// Cindy, change paths to local in order to compile this.
// To compile you use `go build`
// And you can run the test using `go test`

// #include "shamap.h"
// #include <stdlib.h>
import "C"
import (
	"encoding/hex"
	"log"
	"strings"
	"unsafe"
)

type GoSHAMap struct {
	shamap C.SHAMap
}

func MakeSHAMapState() GoSHAMap {
	var ret GoSHAMap
	ret.shamap = C.SHAMapStateInit()
	return ret
}

func MakeSHAMapTxMeta() GoSHAMap {
	var ret GoSHAMap
	ret.shamap = C.SHAMapTxMetaInit()
	return ret
}

func SHAMapTest() {
	C.SHAMapTest()
}

func (shamap GoSHAMap) Free() {
	C.SHAMapFree((C.SHAMap)(unsafe.Pointer(shamap.shamap)))
}

func (shamap GoSHAMap) AddStateItem(key []byte, value1 []byte, size1 uint32) {
	cKey := C.CBytes(key)
	cValue1 := C.CBytes(value1)
	C.SHAMapAddStateItem(shamap.shamap, (*C.char)(cKey), (*C.char)(cValue1), C.uint(size1))
	C.free(cKey)
	C.free(cValue1)
}

func (shamap GoSHAMap) AddTxItem(value1 []byte, size1 uint32, value2 []byte, size2 uint32) {
	if size1 == 0 || size2 == 0 {
		log.Fatalf("Invalid input: value1 and value2 must be non-empty")
	}
	cValue1 := C.CBytes(value1)
	cValue2 := C.CBytes(value2)
	C.SHAMapAddTxItem(shamap.shamap, (*C.char)(cValue1), C.uint(size1), (*C.char)(cValue2), C.uint(size2))
	C.free(cValue1)
	C.free(cValue2)
}

func (shamap GoSHAMap) UpdateStateItem(key []byte, value []byte, size uint32) {
	cKey := C.CBytes(key)
	cValue := C.CBytes(value)
	C.SHAMapUpdateStateItem(shamap.shamap, (*C.char)(cKey), (*C.char)(cValue), C.uint(size))
	C.free(cKey)
	C.free(cValue)
}

func (shamap GoSHAMap) DeleteKey(key []byte) {
	cKey := C.CBytes(key)
	C.SHAMapDeleteKey(shamap.shamap, (*C.char)(cKey))
	C.free(cKey)
}

func (shamap GoSHAMap) GetHash() string {
	//32 is 256/sizeof(char)
	cstr := make([]C.char, 32)
	C.SHAMapGetHash256(shamap.shamap, (*C.char)(unsafe.Pointer(&cstr[0])))
	cbytes := make([]byte, 32)
	for i := range cbytes {
		cbytes[i] = byte(cstr[i])
	}
	return strings.ToUpper(hex.EncodeToString(cbytes))
}
