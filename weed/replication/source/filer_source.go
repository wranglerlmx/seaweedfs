package source

import (
	"io"
	"github.com/chrislusf/seaweedfs/weed/util"
	"github.com/chrislusf/seaweedfs/weed/pb/filer_pb"
	"fmt"
	"github.com/chrislusf/seaweedfs/weed/glog"
	"strings"
	"context"
)

type ReplicationSource interface {
	ReadPart(part string) io.ReadCloser
}

type FilerSource struct {
	grpcAddress string
	id          string
	dir         string
}

func (fs *FilerSource) Initialize(configuration util.Configuration) error {
	return fs.initialize(
		configuration.GetString("grpcAddress"),
		configuration.GetString("id"),
		configuration.GetString("directory"),
	)
}

func (fs *FilerSource) initialize(grpcAddress string, id string, dir string) (err error) {
	fs.grpcAddress = grpcAddress
	fs.id = id
	fs.dir = dir
	return nil
}

func (fs *FilerSource) ReadPart(part string) (readCloser io.ReadCloser, err error) {

	vid2Locations := make(map[string]*filer_pb.Locations)

	vid := volumeId(part)

	err = fs.withFilerClient(func(client filer_pb.SeaweedFilerClient) error {

		glog.V(4).Infof("read lookup volume id locations: %v", vid)
		resp, err := client.LookupVolume(context.Background(), &filer_pb.LookupVolumeRequest{
			VolumeIds: []string{vid},
		})
		if err != nil {
			return err
		}

		vid2Locations = resp.LocationsMap

		return nil
	})

	if err != nil {
		glog.V(1).Infof("replication lookup volume id: %v", vid, err)
		return nil, fmt.Errorf("replicationlookup volume id %v: %v", vid, err)
	}

	locations := vid2Locations[vid]

	if locations == nil || len(locations.Locations) == 0 {
		glog.V(1).Infof("replication locate volume id: %v", vid, err)
		return nil, fmt.Errorf("replication locate volume id %v: %v", vid, err)
	}

	fileUrl := fmt.Sprintf("http://%s/%s", locations.Locations[0].Url, part)

	_, readCloser, err = util.DownloadUrl(fileUrl)

	return readCloser, err
}

func (fs *FilerSource) withFilerClient(fn func(filer_pb.SeaweedFilerClient) error) error {

	grpcConnection, err := util.GrpcDial(fs.grpcAddress)
	if err != nil {
		return fmt.Errorf("fail to dial %s: %v", fs.grpcAddress, err)
	}
	defer grpcConnection.Close()

	client := filer_pb.NewSeaweedFilerClient(grpcConnection)

	return fn(client)
}

func volumeId(fileId string) string {
	lastCommaIndex := strings.LastIndex(fileId, ",")
	if lastCommaIndex > 0 {
		return fileId[:lastCommaIndex]
	}
	return fileId
}