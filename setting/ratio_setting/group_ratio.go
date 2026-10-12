package ratio_setting

import (
	"errors"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
)

var defaultGroupRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":    1,
}

var groupRatioMap = types.NewRWMap[string, float64]()

var defaultGroupGroupRatio = map[string]map[string]float64{}

var groupGroupRatioMap = types.NewRWMap[string, map[string]float64]()

var defaultGroupSpecialUsableGroup = map[string]map[string]string{}

type GroupRatioSetting struct {
	GroupRatio              *types.RWMap[string, float64]            `json:"group_ratio"`
	GroupGroupRatio         *types.RWMap[string, map[string]float64] `json:"group_group_ratio"`
	GroupSpecialUsableGroup *types.RWMap[string, map[string]string]  `json:"group_special_usable_group"`
}

var groupRatioSetting GroupRatioSetting

func init() {
	groupSpecialUsableGroup := types.NewRWMap[string, map[string]string]()
	groupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)

	groupRatioMap.AddAll(defaultGroupRatio)
	groupGroupRatioMap.AddAll(defaultGroupGroupRatio)

	groupRatioSetting = GroupRatioSetting{
		GroupSpecialUsableGroup: groupSpecialUsableGroup,
		GroupRatio:              groupRatioMap,
		GroupGroupRatio:         groupGroupRatioMap,
	}

	config.GlobalConfig.Register("group_ratio_setting", &groupRatioSetting)
}

func GetGroupRatioSetting() *GroupRatioSetting {
	if groupRatioSetting.GroupSpecialUsableGroup == nil {
		groupRatioSetting.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
		groupRatioSetting.GroupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)
	}
	return &groupRatioSetting
}

func GetGroupRatioCopy() map[string]float64 {
	return groupRatioMap.ReadAll()
}

func ContainsGroupRatio(name string) bool {
	_, ok := groupRatioMap.Get(name)
	return ok
}

func GroupRatio2JSONString() string {
	return groupRatioMap.MarshalJSONString()
}

func UpdateGroupRatioByJSONString(jsonStr string) error {
	if err := CheckGroupRatio(jsonStr); err != nil {
		return err
	}
	return types.LoadFromJsonString(groupRatioMap, jsonStr)
}

func GetGroupRatio(name string) float64 {
	ratio, ok := groupRatioMap.Get(name)
	if !ok {
		common.SysLog(common.LogText("group ratio not found: %s", name))
		return 1
	}
	return ratio
}

func GetGroupGroupRatio(userGroup, usingGroup string) (float64, bool) {
	gp, ok := groupGroupRatioMap.Get(userGroup)
	if !ok {
		return -1, false
	}
	ratio, ok := gp[usingGroup]
	if !ok {
		return -1, false
	}
	return ratio, true
}

func GroupGroupRatio2JSONString() string {
	return groupGroupRatioMap.MarshalJSONString()
}

func UpdateGroupGroupRatioByJSONString(jsonStr string) error {
	if strings.TrimSpace(jsonStr) == "" {
		groupGroupRatioMap = types.NewRWMap[string, map[string]float64]()
		groupRatioSetting.GroupGroupRatio = groupGroupRatioMap
		return nil
	}
	if err := CheckGroupGroupRatio(jsonStr); err != nil {
		return err
	}
	return types.LoadFromJsonString(groupGroupRatioMap, jsonStr)
}

func CheckGroupGroupRatio(jsonStr string) error {
	groupRatios := make(map[string]map[string]float64)
	if err := common.UnmarshalJsonStr(jsonStr, &groupRatios); err != nil {
		return err
	}
	for userGroup, ratios := range groupRatios {
		if strings.TrimSpace(userGroup) == "" {
			return errors.New("user group name cannot be empty")
		}
		for serviceGroup, ratio := range ratios {
			if strings.TrimSpace(serviceGroup) == "" {
				return errors.New("service group name cannot be empty")
			}
			if ratio < 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
				return errors.New("special group ratio must be a finite non-negative number: " + userGroup + " -> " + serviceGroup)
			}
		}
	}
	return nil
}

func CheckGroupRatio(jsonStr string) error {
	checkGroupRatio := make(map[string]float64)
	err := common.UnmarshalJsonStr(jsonStr, &checkGroupRatio)
	if err != nil {
		return err
	}
	if len(checkGroupRatio) == 0 {
		return errors.New("at least one service group is required")
	}
	for name, ratio := range checkGroupRatio {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(name) != name || name == "auto" {
			return errors.New("invalid service group name: " + name)
		}
		if ratio < 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
			return errors.New("group ratio must be not less than 0: " + name)
		}
	}
	return nil
}
