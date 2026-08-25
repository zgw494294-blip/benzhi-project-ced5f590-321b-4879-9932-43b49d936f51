package domain

import "time"

func (c *RestorationCase) Freeze(manifest FrozenManifest, now time.Time) error {
	if err := c.CanFreeze(); err != nil {
		return err
	}
	if c.Frozen != nil {
		return NewError(CodeImmutable, "作业版本已经冻结")
	}
	if manifest.CaseID != c.ID || manifest.ContentDigest == "" || manifest.CanonicalJSON == "" {
		return NewError(CodeValidation, "冻结清单不完整")
	}
	manifest.FrozenVersion = c.Version + 1
	manifest.FrozenAt = now.UTC()
	c.Frozen = &manifest
	c.Status = StatusFrozen
	c.bump(now)
	return nil
}

func (c *RestorationCase) Release(permit ReleasePermit, now time.Time) error {
	if c.Status != StatusFrozen || c.Frozen == nil {
		return NewError(CodeState, "只有冻结版本可以批准放行")
	}
	if c.Permit != nil {
		return NewError(CodeImmutable, "该冻结版本已经签发凭据")
	}
	if permit.CaseID != c.ID || permit.FrozenVersion != c.Frozen.FrozenVersion || permit.ContentDigest != c.Frozen.ContentDigest {
		return NewError(CodeValidation, "凭据与冻结版本不匹配")
	}
	if permit.SerialNumber < 1 || permit.ApprovedBy == "" {
		return NewError(CodeValidation, "凭据序号和批准人不能为空")
	}
	permit.IssuedAt = now.UTC()
	c.Permit = &permit
	c.Status = StatusReleased
	c.bump(now)
	return nil
}
