package hkdf

import (
	"encoding/base64"
	"reflect"
	"testing"
)

func TestExpandWithoutAppInfo(t *testing.T) {
	key := []byte{234, 251, 174, 136, 125, 202, 8, 83, 61, 238, 64, 117, 205, 204, 199, 37, 49, 39, 150, 130, 186, 35, 249, 2, 22, 198, 145, 246, 130, 117, 99, 3}
	sswanted := []byte{51, 105, 5, 41, 180, 171, 233, 51, 36, 111, 159, 57, 119, 101, 83, 243, 69, 61, 113, 91, 209, 242, 237, 201, 128, 153, 172, 54, 28, 137, 165, 21, 127, 176, 173, 41, 129, 215, 85, 32, 109, 203, 24, 25, 155, 132, 220, 122, 150, 162, 92, 124, 152, 10, 98, 75, 120, 18, 13, 130, 121, 14, 128, 78, 222, 105, 21, 28, 217, 253, 175, 142, 25, 218, 201, 18, 194, 203, 70, 89}
	ss, err := Expand(key, 80, "")
	if err != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(sswanted, ss) {
		t.Fail()
	}
}

func TestExpandWithAppInfo(t *testing.T) {
	//AppInfo
	ss, err := Expand([]byte("Hallo ich bin Marcel"), 112, "WhatsApp Image Keys")
	if err != nil {
		t.Fail()
	}
	if base64.StdEncoding.EncodeToString(ss) != "P223XLP+ocURAtdHfZYjoQ7IoHQL+eH4yKIVklTfv9q6F/f70wGj5kNPUcEtX1jcIHmsUQUrVGdRCs2DRScgxGyqOcDbzaenRbLkcrvp1upO/wi1iaIuG3MTvdnfuULtJX9LKfkBMrXT683j+twJ2A==" {
		t.Fail()
	}
}
