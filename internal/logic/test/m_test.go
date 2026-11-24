package test

import (
	"authRpc/internal/config"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/zeromicro/go-zero/core/conf"
	"os"
	"testing"
	"time"
)

func CreateSvcCtx() *svc.ServiceContext {
	var c config.Config
	conf.MustLoad(*flag.String("f", "../../../etc/auth_rpc.yaml", "the config file"), &c)
	ctx, err := svc.NewServiceContext(context.Background(), c)
	if err != nil {
		panic(err)
	}
	return ctx
}

func TestTime123(t *testing.T) {
	// 保留项目原有测试入口；其验证的角色鉴权接口已删除，不再连接真实资源调用旧接口。
	t.Skip("角色鉴权接口已移除，用户独立权限鉴权待另行实现")
}
func TestTimeNow(t *testing.T) {

	svctx := CreateSvcCtx()
	defer svctx.Close()

	md, err := mysql.NewMenuModel(context.Background(), svctx)
	if err != nil {
		t.Fatal(err)
	}
	mp := make(map[string]interface{})
	data, err := os.ReadFile("./123.json")
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(data, &mp)
	if err != nil {
		panic(err)
	}
	//
	//data0 := &schema.Menu{
	//	MenuPid:      0,
	//	MenuName:     "接口权限",
	//	MenuType:     3,
	//	Path:         "",
	//	Status:       1,
	//	Sort:         0,
	//	CreatedAt:    time.Now().Unix(),
	//	UpdatedAt:    0,
	//	DeletedAt:    0,
	//	BusinessCode: "99999999",
	//}
	//md.InsertSchema(data0)
	//大类key:vip

	for k, v := range mp {
		if k == "swagger" {
			continue
		}
		data1 := &schema.Menu{
			MenuPid:      145,
			MenuName:     k,
			MenuType:     3,
			Path:         "",
			Status:       1,
			Sort:         0,
			CreatedAt:    time.Now().Unix(),
			UpdatedAt:    0,
			DeletedAt:    0,
			BusinessCode: "99999999",
		}
		md.InsertSchema(data1)
		fmt.Println("大类key:" + k)
		if k == "vip" {
			fmt.Println(12)
		}

		if v == nil {
			continue
		}

		for _, v1 := range v.(map[string]interface{}) {
			if v1 == nil {
				continue
			}
			tv1 := v1.(map[string]interface{})
			if tv1["post"] == nil {
				for _, v3 := range tv1 {
					tv3 := v3.(map[string]interface{})
					if tv3["post"] == nil {
						for _, v5 := range tv3 {
							tv5 := v5.(map[string]interface{})
							if tv5["url"] == nil {
								for _, v6 := range tv5 {
									tv6 := v6.(map[string]interface{})

									if tv6["post"] == nil {
										data2 := &schema.Menu{
											MenuPid:      data1.ID,
											MenuName:     tv6["name"].(string),
											MenuType:     3,
											Path:         tv6["url"].(string),
											Status:       1,
											Sort:         0,
											CreatedAt:    time.Now().Unix(),
											UpdatedAt:    0,
											DeletedAt:    0,
											BusinessCode: "99999999",
										}
										md.InsertSchema(data2)
										fmt.Println("tv7", tv6["url"])
										fmt.Println("tv7", tv6["name"])
										continue
									}
									tv6p := tv6["post"].(map[string]interface{})
									data2 := &schema.Menu{
										MenuPid:      data1.ID,
										MenuName:     tv6p["name"].(string),
										MenuType:     3,
										Path:         tv6p["url"].(string),
										Status:       1,
										Sort:         0,
										CreatedAt:    time.Now().Unix(),
										UpdatedAt:    0,
										DeletedAt:    0,
										BusinessCode: "99999999",
									}
									md.InsertSchema(data2)
									fmt.Println("tv6", tv6p["url"])
									fmt.Println("tv6", tv6p["name"])
								}
								continue
							}
							tv5p := tv5["post"].(map[string]interface{})
							data2 := &schema.Menu{
								MenuPid:      data1.ID,
								MenuName:     tv5p["name"].(string),
								MenuType:     3,
								Path:         tv5p["url"].(string),
								Status:       1,
								Sort:         0,
								CreatedAt:    time.Now().Unix(),
								UpdatedAt:    0,
								DeletedAt:    0,
								BusinessCode: "99999999",
							}
							md.InsertSchema(data2)
							fmt.Println("tv5", tv5p["url"])
							fmt.Println("tv5", tv5p["name"])
						}
						continue
					}
					tv4 := tv3["post"].(map[string]interface{})

					data2 := &schema.Menu{
						MenuPid:      data1.ID,
						MenuName:     tv4["name"].(string),
						MenuType:     3,
						Path:         tv4["url"].(string),
						Status:       1,
						Sort:         0,
						CreatedAt:    time.Now().Unix(),
						UpdatedAt:    0,
						DeletedAt:    0,
						BusinessCode: "99999999",
					}
					md.InsertSchema(data2)
					fmt.Println("tv4", tv4["url"])
					fmt.Println("tv4", tv4["name"])
				}
				continue
			}
			tv2 := tv1["post"].(map[string]interface{})
			if tv2 == nil {
				continue
			}
			fmt.Println("tv2", tv2["url"])
			fmt.Println("tv2", tv2["name"])

			data2 := &schema.Menu{
				MenuPid:      data1.ID,
				MenuName:     tv2["name"].(string),
				MenuType:     3,
				Path:         tv2["url"].(string),
				Status:       1,
				Sort:         0,
				CreatedAt:    time.Now().Unix(),
				UpdatedAt:    0,
				DeletedAt:    0,
				BusinessCode: "99999999",
			}
			md.InsertSchema(data2)
		}
	}

}
