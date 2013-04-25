package soap

import (
	"fmt"
)

type Fault struct {
	Code   string `xml:"faultcode"`
	String string `xml:"faultstring"`
	Actor  string `xml:"faultactor"`
	Detail string `xml:"detail"`
}

func (f *Fault) Error() string {
	return fmt.Sprintf(
		"hiperus: SOAP fault: %s: %s: %s: %s",
		f.Code, f.String, f.Actor, f.Detail,
	)
}
