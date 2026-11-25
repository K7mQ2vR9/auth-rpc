-- 全auth-rpc改用version乐观锁；2026-11-25已在user_rpc执行并核对，当前库禁止重跑。
-- 前置：001—007已完成，停止旧auth-rpc写入实例及相关入口，备份menu/menu_group和目录锁表的结构与数据。
-- 确认menu/menu_group均无version及下列CHECK；007活跃路径列/唯一索引保持，目录锁表仅固定id=1元数据行。
-- 旧客户端必须先适配版本读取及显式回传；新增版本1，修改/删除旧版本或缺省版本不兼容。
-- DDL分阶段隐式提交，任一步失败核对实际阶段，不能用ROLLBACK恢复或盲目从头重跑。

ALTER TABLE `menu`
  ADD COLUMN `version` BIGINT UNSIGNED NOT NULL DEFAULT 1
    COMMENT '菜单乐观锁版本，初始1，修改和软删除按旧版本条件写入并加1，不额外显式锁行'
    AFTER `updated_at`,
  ADD CONSTRAINT `chk_menu_version` CHECK (`version` BETWEEN 1 AND 9223372036854775807);

ALTER TABLE `menu_group`
  ADD COLUMN `version` BIGINT UNSIGNED NOT NULL DEFAULT 1
    COMMENT '展示分组乐观锁版本，初始1，修改和软删除按旧版本条件写入并加1'
    AFTER `updated_at`,
  ADD CONSTRAINT `chk_menu_group_version` CHECK (`version` BETWEEN 1 AND 9223372036854775807);

-- 仅在确认锁表仍只有id=1元数据且旧代码已停用后退役，不删除菜单/分组/模板/账号/授权或审计。
DROP TABLE `permission_catalog_lock`;

-- 保留007的active_route_path与uniq_active_service_path，空路径/软删除释放规则不变。
-- role_permission与admin_permission沿用原version，无新增版本字段；源码移除全部显式悲观锁。
-- 不修改其他业务服务管理的账号结构或创建账号，不自动裁剪CSV、增加模板/用户版本或补授权。
-- 部署须先核对两个版本列/启用CHECK和原路径唯一约束，并确认锁表已退役；随后启用新客户端及新源码。
