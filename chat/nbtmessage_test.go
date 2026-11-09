package chat_test

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/KonjacBot/go-mc/chat"
	en_us "github.com/KonjacBot/go-mc/data/lang/en-us"
	"github.com/KonjacBot/go-mc/nbt"
)

func TestMessage_ReadFrom_CapturedRaidBossEventTitle(t *testing.T) {
	const capturedPacketData = "" +
		"Jhfh/MCpSPuQaWX7YyomIAAKCQAFZXh0cmEKAAAAAggAAAADIC0gAAkABHdpdGgDAAAAAQAAAAII" +
		"AAl0cmFuc2xhdGUAJmV2ZW50Lm1pbmVjcmFmdC5yYWlkLnJhaWRlcnNfcmVtYWluaW5nAAgACXRy" +
		"YW5zbGF0ZQAUZXZlbnQubWluZWNyYWZ0LnJhaWQAPIZO1AICAA=="

	packetData, err := base64.StdEncoding.DecodeString(capturedPacketData)
	if err != nil {
		t.Fatal(err)
	}
	if len(packetData) != 151 {
		t.Fatalf("captured packet data length = %d, want 151", len(packetData))
	}

	// Clientbound packet 9 is BossEvent. Its first 16 bytes are the UUID and
	// the next byte is the Add operation; the title component follows.
	reader := bytes.NewReader(packetData[17:])
	var message chat.Message
	n, err := message.ReadFrom(reader)
	if err != nil {
		t.Fatalf("decode captured BossEvent title: %v", err)
	}
	if n != int64(len(packetData)-17-7) || reader.Len() != 7 {
		t.Fatalf("decoded %d title bytes with %d left, want 7 BossEvent fields", n, reader.Len())
	}
	if message.Translate != "event.minecraft.raid" {
		t.Fatalf("boss title translation = %q, want event.minecraft.raid", message.Translate)
	}
	if len(message.Extra) != 2 {
		t.Fatalf("boss title extras = %#v, want two components", message.Extra)
	}
	remaining := message.Extra[1]
	if remaining.Translate != "event.minecraft.raid.raiders_remaining" {
		t.Fatalf("remaining translation = %q", remaining.Translate)
	}
	if len(remaining.With) != 1 || remaining.With[0] != int32(2) {
		t.Fatalf("remaining translation arguments = %#v, want []any{int32(2)}", remaining.With)
	}
}

func TestMessage_ReadFrom_TranslateIntList(t *testing.T) {
	packetData, err := hex.DecodeString("0a080005636f6c6f720003726564090004776974680300000001000000ff0800097472616e736c617465000d6275696c642e746f6f486967680001")
	if err != nil {
		t.Fatal(err)
	}

	r := bytes.NewReader(packetData)
	var message chat.Message
	n, err := message.ReadFrom(r)
	if err != nil {
		t.Fatalf("decode system chat content: %v", err)
	}
	if n != int64(len(packetData)-1) {
		t.Fatalf("decoded %d bytes, want %d", n, len(packetData)-1)
	}
	if overlay, err := r.ReadByte(); err != nil || overlay != 1 {
		t.Fatalf("overlay byte = %d, %v; want 1", overlay, err)
	}
	if len(message.With) != 1 || message.With[0] != int32(255) {
		t.Fatalf("translation args = %#v, want []any{int32(255)}", message.With)
	}
	if got := message.ClearString(); got != "Height limit for building is 255" {
		t.Fatalf("message = %q", got)
	}
	if got := message.String(); got != "\x1b[91mHeight limit for building is 255\x1b[0m" {
		t.Fatalf("styled message = %q", got)
	}
}

func TestMessage_TranslateRoundTripHasSingleRootTag(t *testing.T) {
	want := chat.TranslateMsg("container.chest")
	var encoded bytes.Buffer
	if _, err := want.WriteTo(&encoded); err != nil {
		t.Fatal(err)
	}
	data := encoded.Bytes()
	if len(data) < 2 {
		t.Fatalf("encoded message is %d bytes", len(data))
	}
	if data[0] != nbt.TagCompound {
		t.Fatalf("root tag = %d, want TagCompound", data[0])
	}
	if data[1] == nbt.TagCompound {
		t.Fatal("message contains a nested unnamed root compound")
	}

	var got chat.Message
	if _, err := got.ReadFrom(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	if got.Translate != want.Translate {
		t.Fatalf("translation = %q, want %q", got.Translate, want.Translate)
	}
}

func TestMessage_UnmarshalJSON_string(t *testing.T) {
	snbts := []string{
		"{translate: sleep.players_sleeping, with: [I; 1, 37]}",
	}

	texts := []string{
		"1/37 players sleeping",
	}

	chat.SetLanguage(en_us.Map)
	for i, v := range snbts {
		bytes, err := nbt.Marshal(nbt.StringifiedMessage(v))
		if err != nil {
			t.Errorf("Invalid SNBT: %v", err)
			continue
		}

		var cm chat.Message
		if err := nbt.Unmarshal(bytes, &cm); err != nil {
			t.Error(err)
		}
		if str := cm.String(); str != texts[i] {
			t.Errorf("gets %q, wants %q", str, texts[i])
		}
	}
}

func TestMessage_UnmarshalNBT_HoverEvent(t *testing.T) {
	type hover struct {
		Action     string         `nbt:"action"`
		ID         string         `nbt:"id,omitempty"`
		Count      int32          `nbt:"count,omitempty"`
		Components map[string]any `nbt:"components,omitempty"`
		Value      *chat.Message  `nbt:"value,omitempty"`
	}
	type part struct {
		Text  string `nbt:"text"`
		Hover hover  `nbt:"hover_event"`
	}
	type root struct {
		Text  string `nbt:"text"`
		Extra []part `nbt:"extra"`
	}
	tip := chat.Text("tip")
	in := root{Text: "商品類型：", Extra: []part{
		{Text: "地圖", Hover: hover{Action: "show_item", ID: "minecraft:filled_map", Count: 16, Components: map[string]any{"minecraft:map_id": int32(7)}}},
		{Text: " x 1", Hover: hover{Action: "show_text", Value: &tip}},
	}}
	var encoded bytes.Buffer
	encoder := nbt.NewEncoder(&encoded)
	encoder.NetworkFormat(true)
	if err := encoder.Encode(in, ""); err != nil {
		t.Fatal(err)
	}
	var message chat.Message
	if _, err := message.ReadFrom(&encoded); err != nil {
		t.Fatalf("decode hover_event: %v", err)
	}
	if got := message.ClearString(); got != "商品類型：地圖 x 1" {
		t.Errorf("text = %q", got)
	}
	if len(message.Extra) != 2 || message.Extra[0].Hover == nil || message.Extra[1].Hover == nil {
		t.Fatalf("hover_event not decoded: %+v", message.Extra)
	}
	item := message.Extra[0].Hover
	if item.Action != "show_item" || item.ID != "minecraft:filled_map" || item.Count != 16 {
		t.Errorf("show_item = %+v", item)
	}
	if id, ok := item.Components["minecraft:map_id"].(int32); !ok || id != 7 {
		t.Errorf("show_item components = %#v", item.Components)
	}
	if text := message.Extra[1].Hover; text.Action != "show_text" || text.Value == nil || text.Value.Text != "tip" {
		t.Errorf("show_text = %+v", text)
	}
}

func TestMessage_ReadFrom_clickEvent(t *testing.T) {
	type click struct {
		Action  string `nbt:"action"`
		Command string `nbt:"command,omitempty"`
		URL     string `nbt:"url,omitempty"`
		Value   string `nbt:"value,omitempty"`
	}
	type part struct {
		Text  string `nbt:"text"`
		Click click  `nbt:"click_event"`
	}
	type root struct {
		Text  string `nbt:"text"`
		Extra []part `nbt:"extra"`
	}
	in := root{Text: "", Extra: []part{
		{Text: "[傳送]", Click: click{Action: "run_command", Command: "/tpaccept"}},
		{Text: "[網站]", Click: click{Action: "open_url", URL: "https://example.com"}},
		{Text: "[複製]", Click: click{Action: "copy_to_clipboard", Value: "abc"}},
	}}
	var encoded bytes.Buffer
	encoder := nbt.NewEncoder(&encoded)
	encoder.NetworkFormat(true)
	if err := encoder.Encode(in, ""); err != nil {
		t.Fatal(err)
	}
	var message chat.Message
	if _, err := message.ReadFrom(&encoded); err != nil {
		t.Fatalf("decode click_event: %v", err)
	}
	want := []struct{ action, target string }{{"run_command", "/tpaccept"}, {"open_url", "https://example.com"}, {"copy_to_clipboard", "abc"}}
	if len(message.Extra) != len(want) {
		t.Fatalf("extra = %+v", message.Extra)
	}
	for i, w := range want {
		c := message.Extra[i].Click
		if c == nil || c.Action != w.action || c.Target() != w.target {
			t.Errorf("click %d = %+v, want %s %s", i, c, w.action, w.target)
		}
	}
}
