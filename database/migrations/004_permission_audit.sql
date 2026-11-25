-- 权限变更审计表；2026-11-25已在user_rpc执行并核对，当前库禁止重跑。
-- 根模板创建、更新及按模板覆盖用户已接入审计源码，结构落库不代表业务已测试或联调。
-- 其他环境执行前核对目标数据库、已完成迁移003，且permission_audit不存在；有同名表时先核对结构，禁止盲目重跑。
-- 只创建空表，不创建账号、菜单、模板或授权数据；不删除admin_info.role_id，不改写其他表。
-- 表名与模型一致；InnoDB保证上述权限变更和成功审计使用同一事务，审计失败时一并回滚。

CREATE TABLE `permission_audit` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '权限审计主键，应用限制在int64正数范围',
  `operation` VARCHAR(64) NOT NULL COMMENT '服务端定义的操作名，如create_site_root_template，不接受请求体指定',
  `operator_id` BIGINT UNSIGNED NOT NULL COMMENT '可信上下文对应的操作者ID，事务内查询管理员后记录',
  `operator_account_type` INT NOT NULL COMMENT '操作时账号类型快照，沿用10、11、20、30、40现有定义',
  `operator_business_code` VARCHAR(10) NOT NULL COMMENT '操作时管理员归属编码，与目标站点分别保存',
  `business_code` VARCHAR(10) NOT NULL COMMENT '本次权限变更的目标站点编码',
  `target_type` VARCHAR(32) NOT NULL COMMENT '目标类型：site_root_template根模板，user_permission用户独立授权',
  `target_id` BIGINT UNSIGNED NOT NULL COMMENT '根模板目标使用模板ID，用户授权目标使用用户ID',
  `before_menu_ids` TEXT NOT NULL COMMENT '变更前的规范化具体权限ID逗号集合，创建前为空字符串',
  `after_menu_ids` TEXT NOT NULL COMMENT '变更后的规范化具体权限ID逗号集合，空字符串表示无权限',
  `before_version` BIGINT UNSIGNED NOT NULL COMMENT '变更前目标版本，创建前为0',
  `after_version` BIGINT UNSIGNED NOT NULL COMMENT '变更后目标版本，首次创建为1，其他更新递增1，复制记录用户授权版本',
  `source_template_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '模板复制来源ID，非复制操作为0，不参与实际授权',
  `source_template_version` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '模板复制来源版本，非复制操作为0',
  `trace_id` CHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '当前链路32位小写十六进制Trace ID，复用现有链路，不另生成',
  `created_at` BIGINT NOT NULL DEFAULT 0 COMMENT '记录时间，Unix秒，由应用模型填写',
  PRIMARY KEY (`id`),
  KEY `idx_audit_business_created` (`business_code`, `created_at`, `id`),
  KEY `idx_audit_operator_created` (`operator_id`, `created_at`, `id`),
  KEY `idx_audit_trace_id` (`trace_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='权限成功变更审计，与业务同事务保存快照，不存密码、Token或密钥，不做默认自动清理';
