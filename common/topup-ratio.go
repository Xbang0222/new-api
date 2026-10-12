package common

import (
	"fmt"
	"math"
	"sync"
)

var topupGroupRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":    1,
}
var topupGroupRatioMutex sync.RWMutex

func TopupGroupRatio2JSONString() string {
	topupGroupRatioMutex.RLock()
	defer topupGroupRatioMutex.RUnlock()
	jsonBytes, err := Marshal(topupGroupRatio)
	if err != nil {
		SysError(LogText("error marshalling topup group ratio: %s", err.Error()))
	}
	return string(jsonBytes)
}

func UpdateTopupGroupRatioByJSONString(jsonStr string) error {
	ratios := make(map[string]float64)
	if err := UnmarshalJsonStr(jsonStr, &ratios); err != nil {
		return err
	}
	for group, ratio := range ratios {
		if ratio <= 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
			return fmt.Errorf("top-up ratio must be a finite positive number: %s", group)
		}
	}
	topupGroupRatioMutex.Lock()
	defer topupGroupRatioMutex.Unlock()
	topupGroupRatio = ratios
	return nil
}

func GetTopupGroupRatio(name string) float64 {
	topupGroupRatioMutex.RLock()
	defer topupGroupRatioMutex.RUnlock()
	ratio, ok := topupGroupRatio[name]
	if !ok {
		SysError(LogText("topup group ratio not found: %s", name))
		return 1
	}
	return ratio
}
