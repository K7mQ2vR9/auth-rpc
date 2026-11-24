-- 菜单所属API服务：在已确认的目标库执行，执行前确认menu为空且service_name/idx_service_path不存在。
-- 有历史菜单时须先明确每条记录所属服务，禁止自动填入任意服务名；本脚本不做数据回填。
-- 服务名由API自身配置提供，增改菜单及实际鉴权都必须显式传入，不允许空值通配。
-- 此迁移为一次性DDL，会隐式提交；执行前保存SHOW CREATE TABLE menu结构快照。
ALTER TABLE `menu`
  ADD COLUMN `service_name` VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL
    COMMENT '接口所属API服务名，如admin-api；增改及权限校验必填，区分大小写，不提供默认服务或空值通配'
    AFTER `path`,
  ADD KEY `idx_service_path` (`business_code`, `service_name`, `path`);
