//go:build linux

package station

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/unicode/norm"
)

type Pattern struct {
	Required  []string `json:"required"`
	Component string   `json:"component"`
	Title     string   `json:"title"`
	ReadOnly  bool     `json:"read_only"`
	Question  string   `json:"question"`
	Confirm   string   `json:"confirm"`
	Correct   string   `json:"correct"`
	Cancel    string   `json:"cancel"`
	Repeat    string   `json:"repeat"`
	Success   string   `json:"success"`
	Failure   string   `json:"failure"`
	Unknown   string   `json:"unknown"`
}

func Patterns() map[string]Pattern {
	makePattern := func(title, component string, required []string, read bool) Pattern {
		return Pattern{Required: required, Component: component, Title: title, ReadOnly: read,
			Question: "%sを教えてください。", Confirm: "%s。%sしてよいですか？",
			Correct: "%sに訂正しました。", Cancel: "この作業を取り消しました。",
			Repeat: "%s", Success: "検証用の模擬操作が完了しました。",
			Failure: "操作に失敗しました。", Unknown: "結果を確認できません。再実行せず結果を照会します。"}
	}
	return map[string]Pattern{
		"print":     makePattern("ラベル印刷", "print-confirmation", []string{"product", "lot", "copies"}, false),
		"inventory": makePattern("在庫登録", "weighing", []string{"product", "lot", "weight", "unit"}, false),
		"sale":      makePattern("販売記録", "sale-confirmation", []string{"product", "lot", "weight", "unit", "amount"}, false),
		"stock":     makePattern("在庫照会", "inventory-result", []string{"product"}, true),
	}
}

var fieldOrder = []string{"product", "cut", "lot", "weight", "unit", "copies", "amount"}
var fieldNames = map[string]string{"product": "商品", "cut": "部位", "lot": "ロット", "weight": "重量", "unit": "単位", "copies": "枚数", "amount": "金額"}
var decimal = regexp.MustCompile(`^\d{1,7}(?:\.\d{1,3})?$`)
var lotFormat = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,63}$`)

func validate(fields Fields) error {
	for k, v := range fields {
		if _, ok := fieldNames[k]; !ok {
			return fmt.Errorf("unknown field: %s", k)
		}
		if len([]rune(v)) > 80 || strings.ContainsAny(v, "\n\r\t") {
			return fmt.Errorf("invalid %s", k)
		}
		if v == "" {
			continue
		}
		switch k {
		case "lot":
			if !lotFormat.MatchString(v) {
				return errors.New("ロットは英数字とハイフンで指定してください")
			}
		case "unit":
			if v != "kg" && v != "g" {
				return errors.New("単位はkgまたはgを指定してください")
			}
		case "copies":
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 30 {
				return errors.New("枚数は1〜30の整数で指定してください")
			}
		case "weight", "amount":
			if !decimal.MatchString(v) {
				return fmt.Errorf("%sは正の数で指定してください", fieldNames[k])
			}
			n, _ := strconv.ParseFloat(v, 64)
			if n <= 0 {
				return fmt.Errorf("%sは0より大きい値で指定してください", fieldNames[k])
			}
			if k == "amount" && strings.Contains(v, ".") {
				return errors.New("金額は円単位の整数で指定してください")
			}
		}
	}
	return nil
}

func prepare(s *State, changed []string) {
	p := Patterns()[s.Kind]
	s.Missing = []string{}
	for _, key := range p.Required {
		if s.Fields[key] == "" {
			s.Missing = append(s.Missing, key)
		}
	}
	// An optional weight still requires an explicit unit, including on labels.
	if s.Fields["weight"] != "" && s.Fields["unit"] == "" && !contains(s.Missing, "unit") {
		s.Missing = append(s.Missing, "unit")
	}
	if s.Fields["unit"] != "" && s.Fields["weight"] == "" && !contains(s.Missing, "weight") {
		s.Missing = append(s.Missing, "weight")
	}
	s.View = View{Component: p.Component, Fields: append([]string{}, fieldOrder...), Changed: changed, Mode: "filled"}
	s.Confirmation = nil
	s.OperationID = ""
	if len(s.Missing) > 0 {
		s.Status = "drafting"
		s.Reply = fmt.Sprintf(p.Question, fieldNames[s.Missing[0]])
		return
	}
	if p.ReadOnly {
		// Fixture provenance is explicit; never present these as facility stock.
		s.Status = "done"
		s.Reply = "検証用データ：" + s.Fields["product"] + "の在庫は2ロット、合計12.5kgです。"
		return
	}
	s.Status = "awaiting_confirmation"
	text := fmt.Sprintf(p.Confirm, readback(s.Fields), p.Title)
	s.Confirmation = &Confirmation{ID: newID(), Revision: s.Revision, Text: text}
	s.Reply = text
}

func contains(values []string, key string) bool {
	for _, v := range values {
		if v == key {
			return true
		}
	}
	return false
}
func readback(fields Fields) string {
	parts := []string{}
	for _, k := range fieldOrder {
		if fields[k] != "" {
			parts = append(parts, fieldNames[k]+" "+fields[k])
		}
	}
	return strings.Join(parts, "、")
}

var lotRE = regexp.MustCompile(`(?i)ロット\s*([a-z0-9][a-z0-9-]*)`)
var countRE = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*枚`)
var weightRE = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(kg|g|キログラム|キロ|グラム)`)
var amountRE = regexp.MustCompile(`(\d+)\s*円`)
var ambiguousNumberRE = regexp.MustCompile(`[-−]\s*\d|\d,\d|(^|[^0-9])\.\d|\d[eE]\d|\d+\.\d+\.`)

// Deliberately conservative, not a general Japanese semantic parser. Ambiguous
// requests are questions, never filled with model guesses. Gemini may propose
// values later, but proposals must pass this service and confirmation contract.
func interpret(s State, c Command) (Command, string) {
	t := strings.TrimSpace(norm.NFKC.String(c.Text))
	t = strings.TrimRight(t, "。！!？?")
	c.Action = "question"
	if ambiguousNumberRE.MatchString(t) || strings.Contains(t, "マイナス") {
		return c, "数値を確認してください。重量は0.07kg、枚数は2枚のように指定してください。"
	}
	if t == "もういい" || t == "いいです" {
		return c, "作業を取り消しますか、それとも会話だけ終えますか？"
	}
	if t == "作業を取り消す" || t == "取り消して" || t == "キャンセル" {
		c.Action = "cancel"
		return c, ""
	}
	if t == "はい" || t == "いいね" || t == "お願いします" || t == "確定" {
		if s.Route == "work" && s.Status == "awaiting_confirmation" && c.ConfirmationID != "" {
			c.Action = "confirm"
			return c, ""
		}
		c.Action = "chat"
		return c, "" // no generic approval across a chat boundary
	}
	if t == "もう一度" || t == "聞き直し" {
		c.Action = "repeat"
		return c, ""
	}
	if strings.Contains(t, "分析") || strings.Contains(t, "比較") || strings.Contains(t, "傾向") {
		c.Action = "analysis"
		return c, "分析する期間・施設・指標・比較条件を指定してください。"
	}
	if strings.Contains(t, "ない") || strings.Contains(t, "違う") || strings.Contains(t, "やめ") || strings.Contains(t, "たぶん") || strings.Contains(t, "かも") {
		return c, "変更したい項目と正しい値を教えてください。"
	}
	fields := Fields{}
	// Prefer the corrected clause, but don't guess spoken numbers or omitted units.
	for _, delimiter := range []string{"ではなく", "じゃなく", "でなく"} {
		if i := strings.LastIndex(t, delimiter); i >= 0 {
			t = t[i+len(delimiter):]
		}
	}
	for _, spec := range []struct {
		key string
		re  *regexp.Regexp
	}{{"lot", lotRE}, {"copies", countRE}, {"weight", weightRE}, {"amount", amountRE}} {
		matches := spec.re.FindAllStringSubmatch(t, -1)
		if len(matches) > 1 {
			return c, fieldNames[spec.key] + "を1つに絞って教えてください。"
		}
		if len(matches) == 1 {
			fields[spec.key] = matches[0][1]
			if spec.key == "weight" {
				u := strings.ToLower(matches[0][2])
				if u == "キログラム" || u == "キロ" {
					u = "kg"
				}
				if u == "グラム" {
					u = "g"
				}
				fields["unit"] = u
			}
		}
	}
	for _, p := range []string{"鹿肉", "猪肉"} {
		if strings.Contains(t, p) {
			if fields["product"] != "" {
				return c, "商品を1つに絞って教えてください。"
			}
			fields["product"] = p
		}
	}
	for _, cut := range []string{"肩ロース", "ロース", "ヒレ", "モモ", "バラ", "ミンチ"} {
		if strings.Contains(t, cut) {
			fields["cut"] = cut
			break
		}
	}
	kind := s.Kind
	if strings.Contains(t, "在庫照会") || strings.Contains(t, "在庫を教えて") {
		kind = "stock"
	} else if strings.Contains(t, "在庫登録") {
		kind = "inventory"
	} else if strings.Contains(t, "販売") {
		kind = "sale"
	} else if strings.Contains(t, "ラベル") || strings.Contains(t, "印刷") {
		kind = "print"
	}
	if len(fields) != 0 || kind != s.Kind {
		c.Action, c.Kind, c.Fields = "patch", kind, fields
		return c, ""
	}
	if strings.Contains(t, "訂正") || strings.Contains(t, "ロット") || strings.Contains(t, "枚") || strings.Contains(t, "印刷") {
		return c, "項目と値をもう一度教えてください。"
	}
	c.Action = "chat"
	return c, ""
}
