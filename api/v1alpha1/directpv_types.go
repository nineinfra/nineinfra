package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DriveName is drive name type.
type DriveName string

// NodeID is node ID type.
type NodeID string

// DriveID is drive ID type.
type DriveID string

// DriveState denotes drive status
type DriveState string

const (
	// DriveStatusReady denotes drive is ready for volume schedule.
	DriveStatusReady DriveState = "Ready"

	// DriveStatusLost denotes associated data by FSUUID is lost.
	DriveStatusLost DriveState = "Lost"

	// DriveStatusError denotes drive is in error state to prevent volume schedule.
	DriveStatusError DriveState = "Error"

	// DriveStatusRemoved denotes drive is removed.
	DriveStatusRemoved DriveState = "Removed"

	// DriveStatusMoving denotes drive is moving volumes.
	DriveStatusMoving DriveState = "Moving"
)

// VolumeStatus represents status of a volume.
type VolumeStatus string

// Enum of VolumeStatus type.
const (
	VolumeStatusPending VolumeStatus = "Pending"
	VolumeStatusReady   VolumeStatus = "Ready"
)

// AccessTier denotes access tier.
type AccessTier string

// Enum values of AccessTier type.
const (
	AccessTierDefault AccessTier = "Default"
	AccessTierWarm    AccessTier = "Warm"
	AccessTierHot     AccessTier = "Hot"
	AccessTierCold    AccessTier = "Cold"
)

// VolumeConditionType denotes volume condition. Allows maximum upto 316 chars.
type VolumeConditionType string

// Enum value of VolumeConditionType type.
const (
	VolumeConditionTypeLost VolumeConditionType = "Lost"
)

// VolumeConditionReason denotes volume reason. Allows maximum upto 1024 chars.
type VolumeConditionReason string

// Enum values of VolumeConditionReason type.
const (
	VolumeConditionReasonDriveLost VolumeConditionReason = "DriveLost"
)

// VolumeConditionMessage denotes drive message. Allows maximum upto 32768 chars.
type VolumeConditionMessage string

// Enum values of VolumeConditionMessage type.
const (
	VolumeConditionMessageDriveLost VolumeConditionMessage = "Associated drive was removed. Refer https://github.com/minio/directpv/blob/master/docs/troubleshooting.md"
)

// DriveConditionType denotes drive condition. Allows maximum upto 316 chars.
type DriveConditionType string

// Enum values of DriveConditionType type.
const (
	DriveConditionTypeMountError      DriveConditionType = "MountError"
	DriveConditionTypeMultipleMatches DriveConditionType = "MultipleMatches"
	DriveConditionTypeIOError         DriveConditionType = "IOError"
	DriveConditionTypeRelabelError    DriveConditionType = "RelabelError"
)

// DriveConditionReason denotes the reason for the drive condition type. Allows maximum upto 1024 chars.
type DriveConditionReason string

// Enum values of DriveConditionReason type.
const (
	DriveConditionReasonMountError      DriveConditionReason = "DriveHasMountError"
	DriveConditionReasonMultipleMatches DriveConditionReason = "DriveHasMultipleMatches"
	DriveConditionReasonIOError         DriveConditionReason = "DriveHasIOError"
	DriveConditionReasonRelabelError    DriveConditionReason = "DriveHasRelabelError"
)

// DriveConditionMessage denotes drive message. Allows maximum upto 32768 chars
type DriveConditionMessage string

// Enum values of DriveConditionMessage type.
const (
	DriveConditionMessageIOError DriveConditionMessage = "Drive has I/O error"
)

// InitStatus denotes initialization status
type InitStatus string

const (
	// InitStatusPending denotes that the initialization request is still pending.
	InitStatusPending InitStatus = "Pending"
	// InitStatusProcessed denotes that the initialization request has been processed.
	InitStatusProcessed InitStatus = "Processed"
	// InitStatusError denotes that the initialization request has failed due to an error.
	InitStatusError InitStatus = "Error"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DirectPVNodeList denotes list of nodes.
type DirectPVNodeList struct {
	metav1.TypeMeta `json:",inline"`
	// metdata is the standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata"`
	Items           []DirectPVNode `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:storageversion
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DirectPVNode denotes Node CRD object.
type DirectPVNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   NodeSpec   `json:"spec,omitempty"`
	Status NodeStatus `json:"status"`
}

// NodeSpec represents DirectPV node specification values.
type NodeSpec struct {
	// +optional
	Refresh bool `json:"refresh,omitempty"`
}

// NodeStatus denotes node information.
type NodeStatus struct {
	// +listType=atomic
	Devices []Device `json:"devices"`
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// Device denotes the device information in a drive
type Device struct {
	Name       string `json:"name"`
	ID         string `json:"id"`
	MajorMinor string `json:"majorMinor"`
	Size       uint64 `json:"size"`
	// +optional
	Make string `json:"make,omitempty"`
	// +optional
	FSType string `json:"fsType,omitempty"`
	// +optional
	FSUUID string `json:"fsuuid,omitempty"`
	// +optional
	DeniedReason string `json:"deniedReason,omitempty"`
}

// DriveSpec represents DirectPV drive specification values.
type DriveSpec struct {
	// +optional
	Unschedulable bool `json:"unschedulable,omitempty"`
	// +optional
	Relabel bool `json:"relabel,omitempty"`
}

// DriveStatus denotes drive information.
type DriveStatus struct {
	TotalCapacity     int64             `json:"totalCapacity"`
	AllocatedCapacity int64             `json:"allocatedCapacity"`
	FreeCapacity      int64             `json:"freeCapacity"`
	FSUUID            string            `json:"fsuuid"`
	Status            DriveState        `json:"status"`
	Topology          map[string]string `json:"topology"`
	// +optional
	Make string `json:"make,omitempty"`
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:storageversion
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DirectPVDrive denotes drive CRD object.
type DirectPVDrive struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   DriveSpec   `json:"spec,omitempty"`
	Status DriveStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DirectPVDriveList denotes list of drives.
type DirectPVDriveList struct {
	metav1.TypeMeta `json:",inline"`
	// metdata is the standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata"`
	Items           []DirectPVDrive `json:"items"`
}
