package common

import (
	"fmt"
	"time"
)

type Duration time.Duration

func (d *Duration) UnmarshalText(b []byte) error {
	dd, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}

	if dd < 0 {
		return fmt.Errorf("duration should be positive, but got %s", dd)
	}

	*d = Duration(dd)
	return nil
}
