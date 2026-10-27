package app

import "testing"

func TestBusinessCardVisibilityReferencesAndValidation(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	upload := uploadImage(t, h)
	qr := upload["url"].(string)
	m := TeamMember{ID: "card-person", Name: "测试名片", Role: "商务", Group: "execution", Enabled: true, Phone: "010-12345678", Wechat: "contact_test", WechatQR: qr}
	save := func(member TeamMember) int {
		status, _, _ := h.call(t, "PUT", "/api/admin/company", map[string]any{"teamMembers": []TeamMember{member}}, true, nil)
		return status
	}
	expect(t, save(m), 200)
	status, public, _ := h.call(t, "GET", "/api/public/config", nil, false, nil)
	expect(t, status, 200)
	members := public["company"].(map[string]any)["teamMembers"].([]any)
	person := members[0].(map[string]any)
	if person["wechatQR"] != qr || person["phone"] != m.Phone {
		t.Fatal("card contacts incorrect", person)
	}
	status, _, _ = h.call(t, "GET", qr, nil, false, nil)
	expect(t, status, 200)
	status, _, _ = h.call(t, "DELETE", "/api/admin/uploads/"+upload["id"].(string), nil, true, nil)
	expect(t, status, 409)
	m.Enabled = false
	expect(t, save(m), 200)
	status, _, _ = h.call(t, "GET", qr, nil, false, nil)
	expect(t, status, 404)
	status, _, _ = h.call(t, "DELETE", "/api/admin/uploads/"+upload["id"].(string), nil, true, nil)
	expect(t, status, 409)
	m.Phone = "javascript:alert(1)"
	expect(t, save(m), 400)
	m.Phone = ""
	m.WechatQR = "https://example.com/qr.png"
	expect(t, save(m), 400)
	if _, ok := person["caseIds"]; ok {
		t.Fatal("member case associations should not be exposed")
	}
}
