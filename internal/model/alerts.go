package model

import (
	"fmt"
	"strings"
)

// DeriveAlerts generates alerts from the current remote and folder state.
func DeriveAlerts(remotes []RemoteDeviceStatus, folders []FolderStatus) []Alert {
	alerts := make([]Alert, 0)

	for _, remote := range remotes {
		if remote.Connected {
			continue
		}
		alerts = append(alerts, Alert{
			Severity:  "critical",
			Code:      "REMOTE_DISCONNECTED",
			Message:   fmt.Sprintf("Remote device %s is disconnected", remote.Name),
			SubjectID: remote.ID,
		})
	}

	for _, folder := range folders {
		if strings.EqualFold(folder.State, "error") {
			alerts = append(alerts, Alert{
				Severity:  "critical",
				Code:      "FOLDER_ERROR",
				Message:   fmt.Sprintf("Folder %s reports error state", folder.Label),
				SubjectID: folder.ID,
			})
			continue
		}

		if folder.NeedItems > 0 || folder.NeedBytes > 0 {
			alerts = append(alerts, Alert{
				Severity:  "warn",
				Code:      "FOLDER_OUT_OF_SYNC",
				Message:   fmt.Sprintf("Folder %s has pending sync items", folder.Label),
				SubjectID: folder.ID,
			})
		}
	}

	return alerts
}
