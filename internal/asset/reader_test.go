package asset_test

import (
	"encoding/json"
	"errors"
	"github.com/buildpacks/pack/internal/asset"
	fakes3 "github.com/buildpacks/pack/internal/asset/fakes"
	"github.com/buildpacks/pack/internal/asset/testmocks"
	"github.com/buildpacks/pack/internal/dist"
	"github.com/buildpacks/pack/internal/layer"
	h "github.com/buildpacks/pack/testhelpers"
	"github.com/docker/docker/pkg/archive"
	"github.com/golang/mock/gomock"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
	"io/ioutil"
	"os"
	"testing"
)

func TestReader(t *testing.T) {
	spec.Run(t, "TestReader", testReader, spec.Parallel(), spec.Report(report.Terminal{}))
}

func testReader(t *testing.T, when spec.G, it spec.S) {
	var (
		mockController *gomock.Controller
		mockReadable   *testmocks.MockReadable

		assert = h.NewAssertionManager(t)

		firstAsset = dist.Asset{
			Sha256:  "first-sha256",
			ID:      "first-asset",
			Version: "1.1.1",
			Name:    "First Asset",
			Stacks:  []string{"stack1", "stack2"},
		}
		secondAsset = dist.Asset{
			Sha256:  "second-sha256",
			ID:      "second-asset",
			Version: "2.2.2",
			Name:    "Second Asset",
			Stacks:  []string{"stack1", "stack2"},
		}
		thirdAsset = dist.Asset{
			Sha256:  "third-sha256",
			ID:      "third-asset",
			Version: "3.3.3",
			Name:    "Third Asset",
			Stacks:  []string{"stack1", "stack2"},
		}

		subject asset.Reader
	)
	it.Before(func() {
		mockController = gomock.NewController(t)
		mockReadable = testmocks.NewMockReadable(mockController)
	})
	when("#Read", func() {
		when("no assets or metadata", func() {
			it("returns empty structs", func() {
				subject = asset.NewReader()

				mockReadable.EXPECT().Label("io.buildpacks.asset.layers").Return("", nil)
				assetBlobs, md, err := subject.Read(mockReadable)
				assert.Nil(err)

				assert.Equal(md, dist.AssetMap{})
				var expected []asset.Blob
				assert.Equal(assetBlobs, expected)
			})
		})
		when("Readable object has asset blobs and metadata", func() {
			it("returns blobs and metadata", func() {
				subject = asset.NewReader()

				md := dist.AssetMap{
					"first-sha256":  firstAsset.ToAssetValue("first-diffID"),
					"second-sha256": secondAsset.ToAssetValue("second-diffID"),
					"third-sha256":  thirdAsset.ToAssetValue("third-diffID"),
				}
				lw, err := layer.NewWriterFactory("linux")

				firstAssetBlob, err := fakes3.NewFakeAssetBlobTar("first layer contents", firstAsset, lw)
				assert.Nil(err)

				firstAssetReader, err := firstAssetBlob.Open()
				assert.Nil(err)

				secondAssetBlob, err := fakes3.NewFakeAssetBlobTar("second layer contents", secondAsset, lw)
				assert.Nil(err)

				secondAssetReader, err := secondAssetBlob.Open()
				assert.Nil(err)

				thirdAssetBlob, err := fakes3.NewFakeAssetBlobTar("third layer contents", thirdAsset, lw)
				assert.Nil(err)

				thirdAssetReader, err := thirdAssetBlob.Open()
				assert.Nil(err)

				mdBytes, err := json.Marshal(md)
				assert.Nil(err)

				mockReadable.EXPECT().Label("io.buildpacks.asset.layers").Return(string(mdBytes), nil)
				mockReadable.EXPECT().GetLayer("first-diffID").Return(firstAssetReader, nil)
				mockReadable.EXPECT().GetLayer("second-diffID").Return(secondAssetReader, nil)
				mockReadable.EXPECT().GetLayer("third-diffID").Return(thirdAssetReader, nil)

				blobs, metadata, err := subject.Read(mockReadable)
				assert.Nil(err)

				assert.Equal(metadata, md)

				assert.Equal(len(blobs), 3)
				assert.Equal(blobs[0].AssetDescriptor(), firstAssetBlob.AssetDescriptor())
				AssertBlobContents(t, blobs[0], "first layer contents")

				assert.Equal(blobs[1].AssetDescriptor(), secondAssetBlob.AssetDescriptor())
				AssertBlobContents(t, blobs[1], "second layer contents")

				assert.Equal(blobs[2].AssetDescriptor(), thirdAssetBlob.AssetDescriptor())
				AssertBlobContents(t, blobs[2], "third layer contents")
			})
		})
	})

	when("error cases", func() {
		when("unable to get label", func() {
			it("errors with a helpful message", func() {
				mockReadable.EXPECT().Label("io.buildpacks.asset.layers").Return("", errors.New("error getting label"))
				subject := asset.NewReader()
				_, _, err := subject.Read(mockReadable)
				h.AssertError(t, err, "unable to get asset layers label")
			})
		})

		when("unable to extract asset from layer", func() {
			var tmpDir string
			it.Before(func() {
				var err error
				tmpDir, err = ioutil.TempDir("", "reader-test")
				assert.Nil(err)
			})
			it.After(func() {
				os.RemoveAll(tmpDir)
			})
			it("errors with helpful message", func() {
				subject := asset.NewReader()
				md := dist.AssetMap{
					"first-sha256": firstAsset.ToAssetValue("first-diffID"),
				}

				mdJSON, err := json.Marshal(md)
				assert.Nil(err)

				mockReadable.EXPECT().Label("io.buildpacks.asset.layers").Return(string(mdJSON), nil)

				emptyArchive, err := archive.Tar(tmpDir, archive.Uncompressed)
				assert.Nil(err)

				mockReadable.EXPECT().GetLayer("first-diffID").Return(emptyArchive, nil)

				subject.Read(mockReadable)
			})
		})
	})
}

func AssertBlobContents(t *testing.T, actual dist.Blob, expectedContents string) {
	actualReader, err := actual.Open()
	h.AssertNil(t, err)
	actualContents, err := ioutil.ReadAll(actualReader)
	h.AssertNil(t, err)

	h.AssertEq(t, string(actualContents), expectedContents)
}
