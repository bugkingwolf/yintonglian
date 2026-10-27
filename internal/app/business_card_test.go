package app

import "testing"

func TestBusinessCardVisibilityReferencesAndValidation(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	upload := uploadImage(t, h)
	qr := upload["url"].(string)
	snapshot, err := h.app.Store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	published := snapshot.Cases[0]
	published.Status = "published"
	draft := snapshot.Cases[1]
	draft.Status = "draft"
	if err = putRow(h.app.Store.DB, "cases", published); err != nil {
		t.Fatal(err)
	}
	if err = putRow(h.app.Store.DB, "cases", draft); err != nil {
		t.Fatal(err)
	}
	m := TeamMember{ID: "card-person", Name: "测试名片", Role: "商务", Group: "execution", Enabled: true, Phone: "010-12345678", Wechat: "contact_test", WechatQR: qr, CaseIDs: []string{published.ID, draft.ID}}
	save := func(member TeamMember) int {
		status, _, _ := h.call(t, "PUT", "/api/admin/company", map[string]any{"teamMembers": []TeamMember{member}}, true, nil)
		return status
	}
	expect(t, save(m), 200)
	status, public, _ := h.call(t, "GET", "/api/public/config", nil, false, nil)
	expect(t, status, 200)
	members := public["company"].(map[string]any)["teamMembers"].([]any)
	person := members[0].(map[string]any)
	if person["wechatQR"] != qr || person["phone"] != m.Phone || len(person["caseIds"].([]any)) != 1 || person["caseIds"].([]any)[0] != published.ID {
		t.Fatal("card contacts or publication filtering incorrect", person)
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
	m.WechatQR = ""
	m.CaseIDs = []string{"missing-case"}
	expect(t, save(m), 400)
	m.CaseIDs = []string{published.ID, published.ID}
	expect(t, save(m), 400)
}
