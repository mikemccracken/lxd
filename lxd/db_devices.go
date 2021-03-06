package main

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/lxc/lxd/shared"
)

func dbDevicesAdd(tx *sql.Tx, w string, cID int64, devices shared.Devices) error {
	str1 := fmt.Sprintf("INSERT INTO %ss_devices (%s_id, name, type) VALUES (?, ?, ?)", w, w)
	stmt1, err := tx.Prepare(str1)
	if err != nil {
		return err
	}
	defer stmt1.Close()
	str2 := fmt.Sprintf("INSERT INTO %ss_devices_config (%s_device_id, key, value) VALUES (?, ?, ?)", w, w)
	stmt2, err := tx.Prepare(str2)
	if err != nil {
		return err
	}
	defer stmt2.Close()
	for k, v := range devices {
		t, err := deviceTypeToDbType(v["type"])
		if err != nil {
			return err
		}
		result, err := stmt1.Exec(cID, k, t)
		if err != nil {
			return err
		}
		id64, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("Error inserting device %s into database", k)
		}
		// TODO: is this really int64? we should fix it everywhere if so
		id := int(id64)
		for ck, cv := range v {
			if ck == "type" {
				continue
			}
			if !validDeviceConfig(v["type"], ck, cv) {
				return fmt.Errorf("Invalid device config %s %s\n", ck, cv)
			}
			_, err = stmt2.Exec(id, ck, cv)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func dbDeviceConfigGet(db *sql.DB, id int, isprofile bool) (shared.Device, error) {
	var query string
	var key, value string
	newdev := shared.Device{} // That's a map[string]string
	inargs := []interface{}{id}
	outfmt := []interface{}{key, value}

	if isprofile {
		query = `SELECT key, value FROM profiles_devices_config WHERE profile_device_id=?`
	} else {
		query = `SELECT key, value FROM containers_devices_config WHERE container_device_id=?`
	}

	results, err := dbQueryScan(db, query, inargs, outfmt)

	if err != nil {
		return newdev, err
	}

	for _, r := range results {
		key = r[0].(string)
		value = r[1].(string)
		newdev[key] = value
	}

	return newdev, nil
}

func dbDevicesGet(db *sql.DB, qName string, isprofile bool) (shared.Devices, error) {
	var q string
	if isprofile {
		q = `SELECT profiles_devices.id, profiles_devices.name, profiles_devices.type
			FROM profiles_devices JOIN profiles
			ON profiles_devices.profile_id = profiles.id
   		WHERE profiles.name=?`
	} else {
		q = `SELECT containers_devices.id, containers_devices.name, containers_devices.type
			FROM containers_devices JOIN containers
			ON containers_devices.container_id = containers.id
			WHERE containers.name=?`
	}
	var id, dtype int
	var name, stype string
	inargs := []interface{}{qName}
	outfmt := []interface{}{id, name, dtype}
	results, err := dbQueryScan(db, q, inargs, outfmt)
	if err != nil {
		return nil, err
	}

	devices := shared.Devices{}
	for _, r := range results {
		id = r[0].(int)
		name = r[1].(string)
		stype, err = dbDeviceTypeToString(r[2].(int))
		if err != nil {
			return nil, err
		}
		newdev, err := dbDeviceConfigGet(db, id, isprofile)
		if err != nil {
			return nil, err
		}
		newdev["type"] = stype
		devices[name] = newdev
	}

	return devices, nil
}
