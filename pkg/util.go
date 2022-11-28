package pkg

import (
	"encoding/hex"
	"fmt"
	buconfig "github.com/coreos/butane/config"
	"github.com/coreos/butane/config/common"
	inventoryv1alpha1 "github.com/onmetal/metal-api/apis/inventory/v1alpha1"
	"github.com/pkg/errors"
	"log"
	"net"
	"os"
	"path"
	"strings"
)

func getIPVersion(s string) string {
	if strings.Contains(s, ":") {
		return "ipv6"
	} else {
		return "ipv4"
	}
}

func getLongIPv6(ip net.IP) string {
	dst := make([]byte, hex.EncodedLen(len(ip)))
	_ = hex.Encode(dst, ip)

	longIpv6 := string(dst[0:4]) + ":" +
		string(dst[4:8]) + ":" +
		string(dst[8:12]) + ":" +
		string(dst[12:16]) + ":" +
		string(dst[16:20]) + ":" +
		string(dst[20:24]) + ":" +
		string(dst[24:28]) + ":" +
		string(dst[28:])

	return strings.ReplaceAll(longIpv6, ":", "-")
}

func doesFileExist(fileName string) bool {
	_, err := os.Stat(fileName)
	// check if error is "file not exists"
	if os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}

func renderButane(dataIn []byte) string {
	// render by butane to json
	options := common.TranslateBytesOptions{
		Raw:    true,
		Strict: false,
		Pretty: false,
	}
	options.NoResourceAutoCompression = true
	dataOut, _, err := buconfig.TranslateBytes(dataIn, options)
	if err != nil {
		log.Printf("\nError in ignition rendering.dataIn is : %+v\n", dataIn)
		log.Printf("Error in ignition rendering: %+v", err)
	}
	return string(dataOut)
}

func readIpxeConfFile(part string) ([]byte, error) {
	var ipxeData []byte
	var err error
	ipxeData, err = os.ReadFile(path.Join(DefaultSecretPath, part))
	if err != nil {
		ipxeData, err = os.ReadFile(path.Join(DefaultConfigMapPath, part))
		if err != nil {
			log.Printf("Problem with default secret and configmap #%v ", err)
			return nil, err
		}
	}

	return ipxeData, nil
}

func checkInventoryMac(inventory *inventoryv1alpha1.Inventory, mac string) error {

	uuid := ""
	if inventory.Spec.System != nil && inventory.Spec.System.ID != "" {
		uuid = inventory.Spec.System.ID
	}
	for label, _ := range inventory.Labels {
		if strings.HasPrefix(label, InventoryMacLabelPrefix) {
			inventoryMac := strings.ReplaceAll(label, InventoryMacLabelPrefix, "")
			if inventoryMac == mac {
				return nil
			}
		}
	}

	return errors.New(fmt.Sprintf("Mac %s not found for Inventory %s", mac, uuid))
}