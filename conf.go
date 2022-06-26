package main

import (
	"encoding/json"
	"fmt"
	"github.com/containernetworking/cni/pkg/types"
)

type NetConf struct {
	types.NetConf
	BridgeName string `json:"bridge"`
	BridgeCidr    string `json:"bridgeCidr"`
	ExternalInterface string `json:"externalIf"`
}

func parseNetConf(bytes []byte) (*NetConf, error) {
	conf := &NetConf{}
	if err := json.Unmarshal(bytes, conf); err != nil {
		return nil, fmt.Errorf("failed to parse network config: %v", err)
	}
	return conf, nil
}
