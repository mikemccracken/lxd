package main

import (
	"sync"
	"testing"
)

func mockStartDaemon() (*Daemon, error) {
	d := &Daemon{
		IsMock:                true,
		imagesDownloading:     map[string]chan bool{},
		imagesDownloadingLock: sync.RWMutex{},
	}

	if err := d.Init(); err != nil {
		return nil, err
	}

	// Call this after Init so we have a log object.
	storageConfig := make(map[string]interface{})
	d.Storage = &storageLogWrapper{w: &storageMock{d: d}}
	if _, err := d.Storage.Init(storageConfig); err != nil {
		return nil, err
	}

	return d, nil
}

func Test_config_value_set_empty_removes_val(t *testing.T) {
	d, err := mockStartDaemon()
	if err != nil {
		t.Errorf("daemon, err='%s'", err)
	}
	defer d.Stop()

	if err = d.ConfigValueSet("storage.lvm_vg_name", "foo"); err != nil {
		t.Error("couldn't set value", err)
	}

	val, err := d.ConfigValueGet("storage.lvm_vg_name")
	if err != nil {
		t.Error("Error getting val")
	}
	if val != "foo" {
		t.Error("Expected foo, got ", val)
	}

	err = d.ConfigValueSet("storage.lvm_vg_name", "")
	if err != nil {
		t.Error("error setting to ''")
	}

	val, err = d.ConfigValueGet("storage.lvm_vg_name")
	if err != nil {
		t.Error("Error getting val")
	}
	if val != "" {
		t.Error("Expected '', got ", val)
	}

	valMap, err := d.ConfigValuesGet()
	if err != nil {
		t.Error("Error getting val")
	}
	if key, present := valMap["storage.lvm_vg_name"]; present {
		t.Errorf("un-set key should not be in values map, it is '%v'", key)
	}

}
