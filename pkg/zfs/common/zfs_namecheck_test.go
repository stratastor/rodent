package common

import (
	"strings"
	"testing"
)

func TestDatasetType(t *testing.T) {
	tests := []struct {
		name    string
		dtype   DatasetType
		isDs    bool
		isSnap  bool
		isFs    bool
		isBookm bool
	}{
		{
			name:    "filesystem",
			dtype:   TypeFilesystem,
			isDs:    true,
			isSnap:  false,
			isFs:    true,
			isBookm: false,
		},
		{
			name:    "snapshot",
			dtype:   TypeSnapshot,
			isDs:    true,
			isSnap:  true,
			isFs:    false,
			isBookm: false,
		},
		{
			name:    "bookmark",
			dtype:   TypeBookmark,
			isDs:    false,
			isSnap:  false,
			isFs:    false,
			isBookm: true,
		},
		{
			name:    "volume",
			dtype:   TypeVolume,
			isDs:    true,
			isSnap:  false,
			isFs:    false,
			isBookm: false,
		},
		{
			name:    "invalid",
			dtype:   TypeInvalid,
			isDs:    false,
			isSnap:  false,
			isFs:    false,
			isBookm: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dtype.IsDataset(); got != tt.isDs {
				t.Errorf("IsDataset() = %v, want %v", got, tt.isDs)
			}
			if got := tt.dtype.IsSnapshot(); got != tt.isSnap {
				t.Errorf("IsSnapshot() = %v, want %v", got, tt.isSnap)
			}
			if got := tt.dtype.IsFilesystem(); got != tt.isFs {
				t.Errorf("IsFilesystem() = %v, want %v", got, tt.isFs)
			}
			if got := tt.dtype.IsBookmark(); got != tt.isBookm {
				t.Errorf("IsBookmark() = %v, want %v", got, tt.isBookm)
			}
		})
	}
}

func TestParseDatasetName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *DatasetComponent
		wantErr bool
	}{
		// Valid cases
		{
			name:  "simple filesystem",
			input: "tank/data",
			want: &DatasetComponent{
				Base: "tank/data",
				Type: TypeFilesystem,
			},
		},
		{
			name:  "snapshot",
			input: "tank/data@snap1",
			want: &DatasetComponent{
				Base:     "tank/data",
				Snapshot: "snap1",
				Type:     TypeSnapshot,
			},
		},
		{
			name:  "bookmark",
			input: "tank/data#mark1",
			want: &DatasetComponent{
				Base:     "tank/data",
				Bookmark: "mark1",
				Type:     TypeBookmark,
			},
		},

		// Invalid cases
		{
			name:    "empty name",
			input:   "",
			wantErr: true,
		},
		{
			name:    "empty component",
			input:   "tank//data",
			wantErr: true,
		},
		{
			name:    "trailing slash",
			input:   "tank/data/",
			wantErr: true,
		},
		{
			name:    "leading slash",
			input:   "/tank/data",
			wantErr: true,
		},
		{
			name:    "invalid characters",
			input:   "tank/data$invalid",
			wantErr: true,
		},
		{
			name:    "multiple @ delimiters",
			input:   "tank/data@snap1@snap2",
			wantErr: true,
		},
		{
			name:    "multiple # delimiters",
			input:   "tank/data#mark1#mark2",
			wantErr: true,
		},
		{
			name:    "both @ and # delimiters",
			input:   "tank/data@snap1#mark1",
			wantErr: true,
		},
		{
			name:    "self reference",
			input:   "tank/.",
			wantErr: true,
		},
		{
			name:    "parent reference",
			input:   "tank/..",
			wantErr: true,
		},
		{
			name:    "too deep nesting",
			input:   "tank/" + strings.Repeat("a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p/q/r/s/t/u/v/w/x/y/z/more/", 100) + "levels",
			wantErr: true,
		},
		{
			name:    "name too long",
			input:   strings.Repeat("a/", 128) + "toolong",
			wantErr: true,
		},
		{
			name:    "empty snapshot name",
			input:   "tank/data@",
			wantErr: true,
		},
		{
			name:    "empty bookmark name",
			input:   "tank/data#",
			wantErr: true,
		},
		{
			name:    "invalid snapshot name chars",
			input:   "tank/data@snap/1",
			wantErr: true,
		},
		{
			name:    "invalid bookmark name chars",
			input:   "tank/data#mark/1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDatasetName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDatasetName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if got.Base != tt.want.Base {
				t.Errorf("Base = %v, want %v", got.Base, tt.want.Base)
			}
			if got.Snapshot != tt.want.Snapshot {
				t.Errorf("Snapshot = %v, want %v", got.Snapshot, tt.want.Snapshot)
			}
			if got.Bookmark != tt.want.Bookmark {
				t.Errorf("Bookmark = %v, want %v", got.Bookmark, tt.want.Bookmark)
			}
			if got.Type != tt.want.Type {
				t.Errorf("Type = %v, want %v", got.Type, tt.want.Type)
			}
		})
	}
}

func TestDatasetComponent_String(t *testing.T) {
	tests := []struct {
		name      string
		component *DatasetComponent
		want      string
	}{
		{
			name: "filesystem",
			component: &DatasetComponent{
				Base: "tank/data",
				Type: TypeFilesystem,
			},
			want: "tank/data",
		},
		{
			name: "snapshot",
			component: &DatasetComponent{
				Base:     "tank/data",
				Snapshot: "snap1",
				Type:     TypeSnapshot,
			},
			want: "tank/data@snap1",
		},
		{
			name: "bookmark",
			component: &DatasetComponent{
				Base:     "tank/data",
				Bookmark: "mark1",
				Type:     TypeBookmark,
			},
			want: "tank/data#mark1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.component.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDatasetDepth(t *testing.T) {
	tests := []struct {
		name string
		path string
		want int
	}{
		{
			name: "root dataset",
			path: "tank",
			want: 0,
		},
		{
			name: "single level",
			path: "tank/data",
			want: 1,
		},
		{
			name: "multiple levels",
			path: "tank/data/nested/deep",
			want: 3,
		},
		{
			name: "with snapshot",
			path: "tank/data/nested@snap",
			want: 2,
		},
		{
			name: "with bookmark",
			path: "tank/data/nested#mark",
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetDatasetDepth(tt.path); got != tt.want {
				t.Errorf("GetDatasetDepth() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPoolNameCheck tests pool name validation
func TestPoolNameCheck(t *testing.T) {
	tests := []struct {
		name    string
		pool    string
		wantErr bool
	}{
		{
			name: "valid pool name",
			pool: "tank",
		},
		{
			name: "valid with numbers",
			pool: "pool2",
		},
		{
			name: "valid with special chars",
			pool: "my-pool_01",
		},
		{
			name:    "empty name",
			pool:    "",
			wantErr: true,
		},
		{
			name:    "starts with number",
			pool:    "1pool",
			wantErr: true,
		},
		{
			name:    "reserved word mirror",
			pool:    "mirror",
			wantErr: true,
		},
		{
			name:    "reserved word raidz",
			pool:    "raidz",
			wantErr: true,
		},
		{
			name:    "reserved word draid",
			pool:    "draid",
			wantErr: true,
		},
		{
			name:    "invalid characters",
			pool:    "my@pool",
			wantErr: true,
		},
		{
			name:    "too long",
			pool:    strings.Repeat("a", MaxDatasetNameLen),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PoolNameCheck(tt.pool)
			if (err != nil) != tt.wantErr {
				t.Errorf("PoolNameCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestComponentNameCheck tests individual component validation
func TestComponentNameCheck(t *testing.T) {
	tests := []struct {
		name    string
		comp    string
		wantErr bool
	}{
		{
			name: "valid component",
			comp: "dataset",
		},
		{
			name: "valid with numbers",
			comp: "data01",
		},
		{
			name: "valid with special chars",
			comp: "my-data_set.01",
		},
		{
			name:    "empty component",
			comp:    "",
			wantErr: true,
		},
		{
			name:    "contains slash",
			comp:    "data/set",
			wantErr: true,
		},
		{
			name:    "contains @",
			comp:    "data@snap",
			wantErr: true,
		},
		{
			name:    "contains #",
			comp:    "data#mark",
			wantErr: true,
		},
		{
			name:    "invalid chars",
			comp:    "data$set",
			wantErr: true,
		},
		{
			name:    "too long",
			comp:    strings.Repeat("a", MaxDatasetNameLen),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ComponentNameCheck(tt.comp)
			if (err != nil) != tt.wantErr {
				t.Errorf("ComponentNameCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSnapshotNameCheck tests snapshot name validation
func TestSnapshotNameCheck(t *testing.T) {
	tests := []struct {
		name    string
		snap    string
		wantErr bool
	}{
		{
			name: "valid snapshot",
			snap: "tank/data@snap1",
		},
		{
			name: "valid with special chars",
			snap: "tank/data@my-snap_01",
		},
		{
			name:    "missing @",
			snap:    "tank/data/snap1",
			wantErr: true,
		},
		{
			name:    "empty snapshot name",
			snap:    "tank/data@",
			wantErr: true,
		},
		{
			name:    "multiple @",
			snap:    "tank/data@snap1@snap2",
			wantErr: true,
		},
		{
			name:    "contains #",
			snap:    "tank/data@snap1#mark1",
			wantErr: true,
		},
		{
			name:    "invalid chars in dataset",
			snap:    "tank/$data@snap1",
			wantErr: true,
		},
		{
			name:    "invalid chars in snapshot",
			snap:    "tank/data@snap/1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SnapshotNameCheck(tt.snap)
			if (err != nil) != tt.wantErr {
				t.Errorf("SnapshotNameCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestBookmarkNameCheck tests bookmark name validation
func TestBookmarkNameCheck(t *testing.T) {
	tests := []struct {
		name    string
		mark    string
		wantErr bool
	}{
		{
			name: "valid bookmark",
			mark: "tank/data#mark1",
		},
		{
			name: "valid with special chars",
			mark: "tank/data#my-mark_01",
		},
		{
			name:    "missing #",
			mark:    "tank/data/mark1",
			wantErr: true,
		},
		{
			name:    "empty bookmark name",
			mark:    "tank/data#",
			wantErr: true,
		},
		{
			name:    "multiple #",
			mark:    "tank/data#mark1#mark2",
			wantErr: true,
		},
		{
			name:    "contains @",
			mark:    "tank/data#mark1@snap1",
			wantErr: true,
		},
		{
			name:    "invalid chars in dataset",
			mark:    "tank/$data#mark1",
			wantErr: true,
		},
		{
			name:    "invalid chars in bookmark",
			mark:    "tank/data#mark/1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := BookmarkNameCheck(tt.mark)
			if (err != nil) != tt.wantErr {
				t.Errorf("BookmarkNameCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestMountpointNameCheck tests mountpoint validation
func TestMountpointNameCheck(t *testing.T) {
	tests := []struct {
		name    string
		mountpt string
		wantErr bool
	}{
		{
			name:    "valid mountpoint",
			mountpt: "/mnt/data",
		},
		{
			name:    "root mountpoint",
			mountpt: "/",
		},
		{
			name:    "multiple levels",
			mountpt: "/mnt/data/subset/more",
		},
		{
			name:    "missing leading slash",
			mountpt: "mnt/data",
			wantErr: true,
		},
		{
			name:    "empty path",
			mountpt: "",
			wantErr: true,
		},
		{
			name:    "too long component",
			mountpt: "/mnt/" + strings.Repeat("a", MaxDatasetNameLen),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MountpointNameCheck(tt.mountpt)
			if (err != nil) != tt.wantErr {
				t.Errorf("MountpointNameCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestDatasetNestCheck tests dataset nesting depth validation
func TestDatasetNestCheck(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name: "single level",
			path: "tank/data",
		},
		{
			name: "multiple valid levels",
			path: "tank/data/subset/more",
		},
		{
			name: "with snapshot",
			path: "tank/data/subset@snap",
		},
		{
			name: "with bookmark",
			path: "tank/data/subset#mark",
		},
		{
			name:    "too deep",
			path:    strings.Repeat("level/", MaxDatasetNesting+1) + "data",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DatasetNestCheck(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("DatasetNestCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
