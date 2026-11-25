-- 站点权限模板增量结构；2026-11-25已在user_rpc执行成功，禁止在该库重复执行。
-- 其他环境先核对已完成迁移001、已有menu_ids且无旧角色字段，再按下述前提安排迁移。
-- 执行前核对实际结构并保存SHOW CREATE TABLE role_permission；本文件不创建或切换数据库。
-- 当前迁移按role_permission无任何记录（含软删除记录）设计；有数据时先明确每条模板的站点和类型，另定数据迁移。
-- 不将既有记录自动分类为根模板或自定义模板，不自动创建根模板，不授予菜单，不删除用户role_id。
-- CHECK约束要求MySQL 8.0.16及以上；MySQL DDL隐式提交，不能依赖ROLLBACK撤销，禁止原样重复执行。

ALTER TABLE `role_permission`
  ADD COLUMN `template_type` TINYINT UNSIGNED NOT NULL
    COMMENT '模板用途：1站点根模板、2自定义模板；无默认类型，写入必须明确指定，普通CRUD仅允许2'
    AFTER `menu_ids`,
  ADD COLUMN `version` BIGINT UNSIGNED NOT NULL DEFAULT 1
    COMMENT '模板版本；首次1，修改及软删除时由业务按旧版本比较并递增，不参与权限计算'
    AFTER `template_type`,
  ADD COLUMN `root_business_code` VARCHAR(10)
    GENERATED ALWAYS AS (CASE WHEN `template_type` = 1 THEN `business_code` ELSE NULL END) STORED
    COMMENT '根模板唯一槽位，仅根模板生成站点编码；自定义模板为NULL，软删除不释放槽位',
  ADD UNIQUE KEY `uniq_site_root` (`root_business_code`),
  ADD CONSTRAINT `chk_role_template_type` CHECK (`template_type` IN (1, 2)),
  ADD CONSTRAINT `chk_role_template_business` CHECK
    (`business_code` IS NOT NULL AND CHAR_LENGTH(TRIM(`business_code`)) > 0),
  ADD CONSTRAINT `chk_role_template_version` CHECK (`version` BETWEEN 1 AND 9223372036854775807),
  COMMENT = '站点根权限与自定义权限模板；保存具体菜单ID，用户独立授权，不继承权限组或绑定角色';

-- uniq_site_root仅限制根模板；自定义模板的生成列为NULL，因此同站点可保存多条。
-- 普通接口禁止创建、修改和删除根模板，未来平台专用入口按business_code+template_type识别。
-- 必须先实施本迁移再部署依赖template_type/version的新源码；不重跑001或002，不写入占位根记录。
