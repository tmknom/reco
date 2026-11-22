package setting

// upperASCII は、s の ASCII の英小文字だけを大文字にした綴りを返す。名前が同じかどうかの判定は、この綴りどうしを比べて行う。
// reco が同じ名前と判定できるのは、GitHub が同じとみなすと確かめた ASCII の英字の大文字小文字の違いだけである。
// strings.ToUpper や strings.EqualFold は Unicode の畳み込みで ſ を s と同じとみなし、確かめていない名前まで同じとみなすため使わない。
// バイト単位で変えるのは、不正な UTF-8 のバイトを置き換えず、違う綴りを同じ綴りにしないため。
func upperASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'a' <= c && c <= 'z' {
			b[i] = c - ('a' - 'A')
		}
	}
	return string(b)
}
