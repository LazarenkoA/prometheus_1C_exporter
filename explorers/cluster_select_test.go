package exporter

import (
	"fmt"
	"testing"

	mock_models "github.com/LazarenkoA/prometheus_1C_exporter/explorers/mock"
	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

const (
	firstClusterID  = "2d69c3fa-1bfd-4ce6-b769-1dcfdcb74e85"
	secondClusterID = "dccf2e7a-edda-4dbb-991e-70ed726b1bf9"
)

// вывод "rac cluster list" для агента с двумя кластерами
func testDataTwoClusters() string {
	return "cluster                                   : " + firstClusterID + "\r\n" +
		"host                                      : app1c01\r\n" +
		"port                                      : 1541\r\n" +
		"name                                      : \"Локальный кластер\"\r\n" +
		"expiration-timeout                        : 60\r\n" +
		"\r\n" +
		"cluster                                   : " + secondClusterID + "\r\n" +
		"host                                      : app1c01\r\n" +
		"port                                      : 1640\r\n" +
		"name                                      : \"Дополнительный кластер\"\r\n" +
		"expiration-timeout                        : 60\r\n"
}

func TestSelectCluster(t *testing.T) {
	exp := new(BaseRACExporter)
	exp.logger = logger.NopLogger

	var clusters []map[string]string
	exp.formatMultiResult(testDataTwoClusters(), &clusters)
	assert.Len(t, clusters, 2)

	cases := map[string]string{
		"":              firstClusterID, // по умолчанию - первый кластер
		firstClusterID:  firstClusterID,
		secondClusterID: secondClusterID,
		"Дополнительный кластер": secondClusterID,
		"app1c01:1640": secondClusterID,
		"unknown":      "",
	}
	for want, expected := range cases {
		assert.Equal(t, expected, selectCluster(clusters, want), "RAC.Cluster=%q", want)
	}
}

func TestGetClusterID(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()

	cases := map[string]string{
		"":              firstClusterID,
		secondClusterID: secondClusterID,
		"unknown":       "",
	}
	for want, expected := range cases {
		s := &settings.Settings{}
		err := yaml.Unmarshal([]byte(fmt.Sprintf("RAC:\n  Path: rac\n  Cluster: %q\n", want)), s)
		assert.NoError(t, err)
		assert.Equal(t, want, s.RAC_Cluster())

		run := mock_models.NewMockIRunner(c)
		run.EXPECT().Run(gomock.Any()).Return(testDataTwoClusters(), nil)

		exp := new(ExporterClientLic).Construct(s)
		exp.runner = run

		assert.Equal(t, expected, exp.GetClusterID(), "RAC.Cluster=%q", want)
	}
}
