package quant

import (
	"encoding/json"
	"fmt"
)

func PrettyPrint(format string, T any) {
	prettyPrint, err := json.MarshalIndent(T, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Printf(format, prettyPrint)
}
