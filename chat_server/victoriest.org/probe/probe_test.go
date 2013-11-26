package probe

import (
	"reflect"
	"testing"
)

// 构造struct
func genVictoriestMsg() *VictoriestMsg {
	msgObj := &VictoriestMsg{MsgContext: TestMsg{MsgInt: 55, ChatMessage: "hello world"}}
	return msgObj
}

func testUtil(probe Serializable, t *testing.T) {
	msgObj := genVictoriestMsg()
	// 序列化
	bt, err := probe.Serialize(msgObj, MSG_TYPE_TEST_MESSGAE)
	if err != nil {
		t.Error("error on probe.Serialize")
	}
	t.Log("success on probe.Serialize")

	// 反序列化
	var dest interface{}
	var mst int32
	mst, err = probe.Deserialize(bt, &dest)

	if err != nil {
		t.Error("error on probe.Deserialize")
	}
	// obj, ok := (dest.MsgContext).(TestMsg)
	// if !ok {
	// 	t.Error("dest.(VictoriestMsg) is not ok", obj)
	// }

	t.Log("success on probe.Deserialize  ", reflect.TypeOf(dest), "  ", dest, mst)
}

func TestJsonProbe(t *testing.T) {
	probe := new(JsonProbe)
	testUtil(probe, t)
}

// func TestGobProbe(t *testing.T) {
// 	probe := new(GobProbe)
// 	testUtil(probe, t)
// }
