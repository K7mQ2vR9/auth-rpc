-- 用户独立权限与角色模板迁移：先选择已确认的目标数据库，再执行本文件。
-- 前置条件：role_permission存在，且无记录（包括软删除记录）。
-- 本迁移为一次性旧列转换，执行成功后不可原样重复运行；后续应先核对实际列与索引。
-- 旧permission_ids是权限组ID的JSON数组，不能直接当作菜单ID复制；有数据时须另定转换方案。
-- 本文件不修改admin_info、menu、permission_menu，不创建或切换数据库，不写入用户授权。
-- MySQL DDL会隐式提交；执行前保存角色表结构，不能用ROLLBACK撤销此迁移。

CREATE TABLE IF NOT EXISTS `admin_permission` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '用户权限记录主键；应用使用int64时限制在MaxInt64以内',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '管理员ID，对应admin_info.id；一个用户只保存一条记录，不绑定角色',
  `menu_ids` TEXT NOT NULL COMMENT '英文逗号分隔的具体菜单ID；空字符串表示无权限，写入时显式传入；应用负责校验、去重和排序',
  `business_code` VARCHAR(10) NOT NULL DEFAULT '' COMMENT '权限记录的业务归属；默认空，写入时由业务显式确定',
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '权限记录版本；首次保存为1，后续覆盖保存按版本校验并递增，不参与权限计算',
  `created_at` BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间，Unix秒级时间戳；默认0，首次保存时由应用写入',
  `updated_at` BIGINT NOT NULL DEFAULT 0 COMMENT '最后修改时间，Unix秒级时间戳；默认0，保存时由应用写入',
  `deleted_at` BIGINT NULL DEFAULT NULL COMMENT '软删除时间，Unix秒级时间戳；NULL表示未删除；恢复时复用原用户记录',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_user_id` (`user_id`),
  KEY `idx_business_code` (`business_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='管理员独立菜单权限；模板仅作覆盖复制来源，后续模板变更不联动用户';

-- 角色只保存初始化或覆盖勾选项的菜单快照，不再继承权限组或提供超级角色绕过。
ALTER TABLE `role_permission`
  CHANGE COLUMN `permission_ids` `menu_ids` TEXT NOT NULL
    COMMENT '英文逗号分隔的具体菜单ID；空字符串表示空模板，不再保存权限组ID或JSON数组',
  DROP COLUMN `is_super_role`,
  COMMENT = '角色权限模板；复制后用户权限独立，修改或删除模板不联动用户';
