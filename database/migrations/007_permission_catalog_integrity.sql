-- 菜单目录防环互斥及接口路径唯一约束；2026-11-25已在user_rpc执行并核对，当前库禁止重跑。
-- 前置：001—006已完成，暂停目录写入口，备份menu/menu_group及其结构和数据。
-- 核对permission_catalog_lock、menu.active_route_path及uniq_active_service_path均不存在；
-- 核对service_name仍为utf8mb4_bin（002已定义），business_code/路径/服务名均符合源码长度及无首尾空白规则。
-- 检查所有未删除、非空路径记录在business_code+精确service_name+二进制path上无重复；
-- 检查存量菜单父级无循环/跨目录/缺失，分组成员为正整数JSON数组；发现异常先人工处理，不自动删记录或改授权。
-- DDL隐式提交；任一步失败按实际阶段处理，不从头重跑，不依赖ROLLBACK撤销结构。

ALTER TABLE `menu`
  ADD COLUMN `active_route_path` VARBINARY(1020)
    GENERATED ALWAYS AS
      (CASE WHEN `deleted_at` IS NULL AND OCTET_LENGTH(`path`) > 0
        THEN CAST(`path` AS BINARY) ELSE NULL END) STORED
    COMMENT '未删除且非空路径的UTF8原始字节，最多255字符/1020字节；空路径或软删除为NULL',
  ADD UNIQUE KEY `uniq_active_service_path` (`business_code`, `service_name`, `active_route_path`);

-- service_name的utf8mb4_bin区分大小写；路径VARBINARY精确比较，不使用大小写宽松匹配。
-- 启用与禁用均占用唯一键；空路径和软删除生成NULL不占用，重建菜单必须使用新ID，不继承旧授权。
-- 保留原idx_service_path及其他字段、索引、菜单ID和模板/用户授权，不迁移或初始化业务数据。

CREATE TABLE `permission_catalog_lock` (
  `id` TINYINT UNSIGNED NOT NULL COMMENT '全平台目录互斥行，固定1，事务SELECT FOR UPDATE后方可修改目录',
  PRIMARY KEY (`id`),
  CONSTRAINT `chk_permission_catalog_lock_id` CHECK (`id` = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='菜单与分组维护的事务互斥行，不是业务模板、用户、权限或授权数据';

INSERT INTO `permission_catalog_lock` (`id`) VALUES (1);

-- 只有此正式迁移初始化互斥行，业务请求不建表、不补行；缺失时目录写入失败关闭。
-- 源码依赖本迁移：执行核对前不能部署目录写入口；不操作账号创建、删除、替换或恢复。
