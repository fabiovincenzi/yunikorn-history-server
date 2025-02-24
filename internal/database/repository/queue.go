package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/apache/yunikorn-core/pkg/webservice/dao"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/G-Research/yunikorn-history-server/internal/model"
)

func (s *PostgresRepository) UpsertQueues(ctx context.Context, queues []*dao.PartitionQueueDAOInfo) error {
	upsertSQL := `INSERT INTO queues (
		id,
        parent_id,
		queue_name,
		status,
		partition,
		pending_resource,
		max_resource,
		guaranteed_resource,
		allocated_resource,
		preempting_resource,
		head_room,
		is_leaf,
		is_managed,
		properties,
		parent,
		template_info,
		abs_used_capacity,
		max_running_apps,
		running_apps,
		current_priority,
		allocating_accepted_apps,
    	created_at) VALUES (@id, @parent_id, @queue_name, @status, @partition, @pending_resource, @max_resource,
		@guaranteed_resource, @allocated_resource, @preempting_resource, @head_room, @is_leaf, @is_managed, @properties,
		@parent, @template_info, @abs_used_capacity, @max_running_apps, @running_apps,
		@current_priority, @allocating_accepted_apps, @created_at)
	ON CONFLICT (partition, queue_name) DO UPDATE SET
		status = EXCLUDED.status,
		pending_resource = EXCLUDED.pending_resource,
		max_resource = EXCLUDED.max_resource,
		guaranteed_resource = EXCLUDED.guaranteed_resource,
		allocated_resource = EXCLUDED.allocated_resource,
		preempting_resource = EXCLUDED.preempting_resource,
		head_room = EXCLUDED.head_room,
		is_leaf = EXCLUDED.is_leaf,
		is_managed = EXCLUDED.is_managed,
		max_running_apps = EXCLUDED.max_running_apps,
		running_apps = EXCLUDED.running_apps`
	for _, q := range queues {
		parentId, err := s.getQueueID(ctx, q.Parent, q.Partition)
		if err != nil {
			return fmt.Errorf("could not get parent queue from DB: %v", err)
		}
		_, err = s.dbpool.Exec(ctx, upsertSQL,
			pgx.NamedArgs{
				"id":                       uuid.NewString(),
				"parent_id":                parentId,
				"queue_name":               q.QueueName,
				"status":                   q.Status,
				"partition":                q.Partition,
				"pending_resource":         q.PendingResource,
				"max_resource":             q.MaxResource,
				"guaranteed_resource":      q.GuaranteedResource,
				"allocated_resource":       q.AllocatedResource,
				"preempting_resource":      q.PreemptingResource,
				"head_room":                q.HeadRoom,
				"is_leaf":                  q.IsLeaf,
				"is_managed":               q.IsManaged,
				"properties":               q.Properties,
				"parent":                   q.Parent,
				"template_info":            q.TemplateInfo,
				"abs_used_capacity":        q.AbsUsedCapacity,
				"max_running_apps":         q.MaxRunningApps,
				"running_apps":             q.RunningApps,
				"current_priority":         q.CurrentPriority,
				"allocating_accepted_apps": q.AllocatingAcceptedApps,
				"created_at":               time.Now().Unix(),
			})
		if err != nil {
			return fmt.Errorf("could not insert/update queue into DB: %v", err)
		}
	}
	return nil
}

func (s *PostgresRepository) GetAllQueues(ctx context.Context) ([]*model.PartitionQueueDAOInfo, error) {
	var queues []*model.PartitionQueueDAOInfo
	rows, err := s.dbpool.Query(ctx, "SELECT * FROM queues")
	if err != nil {
		return nil, fmt.Errorf("could not get queues from DB: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var q model.PartitionQueueDAOInfo
		err = rows.Scan(
			&q.Id,
			&q.ParentId,
			&q.CreatedAt,
			&q.DeletedAt,
			&q.QueueName,
			&q.Status,
			&q.Partition,
			&q.PendingResource,
			&q.MaxResource,
			&q.GuaranteedResource,
			&q.AllocatedResource,
			&q.PreemptingResource,
			&q.HeadRoom,
			&q.IsLeaf,
			&q.IsManaged,
			&q.Properties,
			&q.Parent,
			&q.TemplateInfo,
			&q.AbsUsedCapacity,
			&q.MaxRunningApps,
			&q.RunningApps,
			&q.CurrentPriority,
			&q.AllocatingAcceptedApps,
		)
		if err != nil {
			return nil, fmt.Errorf("could not scan queue from DB: %v", err)
		}
		queues = append(queues, &q)
	}
	return queues, nil
}

func (s *PostgresRepository) GetQueuesPerPartition(
	ctx context.Context,
	parition string,
) ([]*model.PartitionQueueDAOInfo, error) {
	selectSQL := `SELECT * FROM queues WHERE partition = $1`

	var queues []*model.PartitionQueueDAOInfo
	childrenMap := make(map[string][]*model.PartitionQueueDAOInfo)

	rows, err := s.dbpool.Query(ctx, selectSQL, parition)
	if err != nil {
		return nil, fmt.Errorf("could not get queues from DB: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var q model.PartitionQueueDAOInfo
		err = rows.Scan(
			&q.Id,
			&q.ParentId,
			&q.CreatedAt,
			&q.DeletedAt,
			&q.QueueName,
			&q.Status,
			&q.Partition,
			&q.PendingResource,
			&q.MaxResource,
			&q.GuaranteedResource,
			&q.AllocatedResource,
			&q.PreemptingResource,
			&q.HeadRoom,
			&q.IsLeaf,
			&q.IsManaged,
			&q.Properties,
			&q.Parent,
			&q.TemplateInfo,
			&q.AbsUsedCapacity,
			&q.MaxRunningApps,
			&q.RunningApps,
			&q.CurrentPriority,
			&q.AllocatingAcceptedApps,
		)
		if err != nil {
			return nil, fmt.Errorf("could not scan queue from DB: %v", err)
		}
		if q.ParentId.Valid {
			childrenMap[q.ParentId.String] = append(childrenMap[q.ParentId.String], &q)
		} else {
			queues = append(queues, &q)
		}
	}
	for _, queue := range queues {
		queue.Children = getChildrenFromMap(queue.Id, childrenMap)
	}
	return queues, nil
}

func (s *PostgresRepository) getQueueID(ctx context.Context, queueName string, partition string) (*string, error) {
	if queueName == "" {
		return nil, nil
	}
	const queueIDSQL = "SELECT id FROM queues WHERE queue_name = $1 AND partition = $2 AND deleted_at IS NULL"
	var id string
	err := s.dbpool.QueryRow(ctx, queueIDSQL, queueName, partition).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("could not get queueName queue from DB: %v", err)
	}
	return &id, nil
}

func getChildrenFromMap(queueID string, childrenMap map[string][]*model.PartitionQueueDAOInfo) []model.PartitionQueueDAOInfo {
	children := childrenMap[queueID]
	var childrenResult []model.PartitionQueueDAOInfo
	for _, child := range children {
		child.Children = getChildrenFromMap(child.Id, childrenMap)
		childrenResult = append(childrenResult, *child)
	}
	return childrenResult
}
