package cli

import (
	"fmt"
	"io"
)

const banner = `
  _  __      _
 | |/ /     | |
 | ' / _ __ | | __ _ _ __   ___
 |  < | '_ \| |/ _` + "`" + ` | '_ \ / _ \
 | . \| |_) | | (_| | | | |  __/
 |_|\_\ .__/|_|\__,_|_| |_|\___|
      | |
      |_|
`

func printBanner(out io.Writer) {
	fmt.Fprintln(out, banner)
}
