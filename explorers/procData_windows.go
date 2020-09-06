// +build windows

package explorer

import (
	"errors"
)



func newProcData() (Iproc, error) {
	return nil, errors.New("ОС windows не поддерживается")
}
