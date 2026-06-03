package mcp

import (
	"fmt"

	"github.com/google/uuid"

	"hornerodb/internal/models/metadata"
	"hornerodb/internal/services/data"
	"hornerodb/internal/services/permission"
)

// fetchHeadRecords returns up to `limit` records of a table, reusing the data service so that
// row-level and column-level security still apply (so resources don't leak data the user can't see).
func (s *Server) fetchHeadRecords(ctx ToolContext, wsID string, table *metadata.Table, accessLevel interface{}, limit int) ([]map[string]interface{}, error) {
	wsUUID, err := uuid.Parse(wsID)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace_id: %v", err)
	}

	al := permission.AccessLevel(fmt.Sprint(accessLevel))
	reqCtx := data.RequestContext{
		WsID:        wsUUID,
		Table:       *table,
		TableName:   "data_" + wsID + "_" + table.Slug,
		UserID:      ctx.UserID,
		RoleName:    ctx.RoleName,
		AccessLevel: al,
	}

	records, _, err := s.dataSvc.ListRecords(reqCtx, limit, 0, "")
	if err != nil {
		return nil, err
	}
	return records, nil
}
