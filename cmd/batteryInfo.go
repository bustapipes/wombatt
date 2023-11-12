package cmd

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"wombatt/internal/batteries"
	"wombatt/internal/common"
	"wombatt/internal/modbus"

	"go.bug.st/serial"
)

type BatteryInfoCmd struct {
	Address     string        `required:"" short:"p" help:"Serial port or address used for communication"`
	IDs         []uint        `required:"" short:"i" name:"battery-ids" help:"IDs of the batteries to get info from."`
	ReadTimeout time.Duration `short:"t" default:"500ms" help:"Timeout when reading from serial ports"`
	BaudRate    uint          `short:"B" default:"9600" help:"Baud rate"`
	BatteryType BatteryType   `default:"EG4LLv2" help:"One of ${battery_types}" enum:"${battery_types}"`
	Protocol    string        `default:"auto" enum:"${protocols}" help:"One of ${protocols}"`
	DeviceType  string        `short:"T" default:"serial" enum:"${device_types}" help:"One of ${device_types}"`
}

func (cmd *BatteryInfoCmd) Run(globals *Globals) error {
	portOptions := &common.PortOptions{
		Address: cmd.Address,
		Mode:    &serial.Mode{BaudRate: int(cmd.BaudRate)},
		Type:    common.DeviceTypeFromString[cmd.DeviceType],
	}
	battery := batteries.Instance(string(cmd.BatteryType))
	if cmd.Protocol == "auto" {
		cmd.Protocol = battery.DefaultProtocol()
	}
	port := common.OpenPortOrFatal(portOptions)
	reader, err := modbus.ReaderFromProtocol(port, cmd.Protocol)
	if err != nil {
		log.Fatal(err.Error())
	}
	var failed error
	for _, id := range cmd.IDs {
		binfo, err := battery.ReadInfo(reader, uint8(id), cmd.ReadTimeout)
		if err != nil {
			failed = errors.Join(failed, fmt.Errorf("error getting info of ID#%d: %w", id, err))
			if err := port.ReopenWithBackoff(); err != nil {
				log.Fatalf("error reopening port: %v", err)
				return err
			}
			continue
		}
		extra, err := battery.ReadExtraInfo(reader, uint8(id), cmd.ReadTimeout)
		if err != nil {
			failed = errors.Join(failed, fmt.Errorf("error getting extra info of ID#%d: %w", id, err))
			if err := port.ReopenWithBackoff(); err != nil {
				log.Fatalf("error reopening port: %v", err)
				return err
			}
			continue
		}
		fmt.Printf("Battery #%d\n===========\n", id)
		writeBatteryInfo(binfo)
		if extra != nil {
			writeBatteryInfo(extra)
		}
		fmt.Println()
	}
	if failed != nil {
		log.Fatal(failed)
	}
	return nil
}

func writeBatteryInfo(bi any) {
	f := func(info map[string]string, value interface{}) {
		name := info["name"]
		unit := info["unit"]
		name = strings.ReplaceAll(name, "_", " ")
		fmt.Printf("%s: %v%s\n", name, value, unit)
	}
	common.TraverseStruct(bi, f)
}
