package github

import "testing"

func sampleRelease() *GHRelease {
	return &GHRelease{
		Tag: "v0.5.1",
		Assets: []GHAsset{
			{ID: 1, Name: "bundle.zip"},
			{ID: 2, Name: "test-deploy-website-v0.5.1-20260815.zip"},
		},
	}
}

// 精确匹配仍优先 (默认配置 asset: bundle.zip)
func TestFindAssetExact(t *testing.T) {
	a, err := FindAsset(sampleRelease(), "bundle.zip", "test-deploy-website", "test-deploy-website")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != 1 {
		t.Fatalf("期望精确匹配 bundle.zip, 得到 %s", a.Name)
	}
}

// 配置名不存在时, 模糊匹配 "项目名-版本tag-日期.zip"
func TestFindAssetFuzzy(t *testing.T) {
	a, err := FindAsset(sampleRelease(), "whatever.zip", "test-deploy-website", "test-deploy-website")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != 2 {
		t.Fatalf("期望模糊匹配版本化资产, 得到 %s", a.Name)
	}
}

// 项目名不匹配 → 找不到
func TestFindAssetNoMatch(t *testing.T) {
	_, err := FindAsset(sampleRelease(), "whatever.zip", "other-project", "other-project")
	if err == nil {
		t.Fatal("期望报错")
	}
}

// 无项目名线索时仅精确匹配 (自更新场景)
func TestFindAssetExactOnly(t *testing.T) {
	a, err := FindAsset(sampleRelease(), "bundle.zip", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != 1 {
		t.Fatalf("期望精确匹配, 得到 %s", a.Name)
	}
}
