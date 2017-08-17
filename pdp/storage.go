package pdp

import "github.com/google/uuid"

type PolicyStorage struct {
	tag      *uuid.UUID
	attrs    map[string]Attribute
	policies Evaluable
}

func NewPolicyStorage(p Evaluable, a map[string]Attribute, t *uuid.UUID) *PolicyStorage {
	return &PolicyStorage{tag: t, attrs: a, policies: p}
}

func (s *PolicyStorage) Attributes() map[string]Attribute {
	attrs := make(map[string]Attribute)
	for ID, a := range s.attrs {
		attrs[ID] = a
	}

	return attrs
}

func (s *PolicyStorage) Root() Evaluable {
	return s.policies
}

func (s *PolicyStorage) CheckTag(tag *uuid.UUID) error {
	if s.tag == nil {
		return newUntaggedPolicyModificationError()
	}

	if tag == nil {
		return newMissingPolicyTagError()
	}

	if s.tag.String() != tag.String() {
		return newPolicyTagsNotMatchError(s.tag, tag)
	}

	return nil
}

func (s *PolicyStorage) NewTransaction(tag *uuid.UUID) (*PolicyStorageTransaction, error) {
	err := s.CheckTag(tag)
	if err != nil {
		return nil, err
	}

	return &PolicyStorageTransaction{tag: *tag, attrs: s.attrs, policies: s.policies}, nil
}

const (
	UOAdd = iota
	UODelete
)

var (
	UpdateOpIDs = map[string]int{
		"add":    UOAdd,
		"delete": UODelete}

	UpdateOpNames = []string{
		"Add",
		"Delete"}
)

type PolicyUpdate struct {
	oldTag uuid.UUID
	newTag uuid.UUID
	cmds   []*command
}

func NewPolicyUpdate(oldTag, newTag uuid.UUID) (*PolicyUpdate, error) {
	return &PolicyUpdate{
		oldTag: oldTag,
		newTag: newTag,
		cmds:   []*command{}}, nil
}

func (u *PolicyUpdate) Append(op int, path []string, entity interface{}) {
	u.cmds = append(u.cmds, &command{op: op, path: path, entity: entity})
}

type command struct {
	op     int
	path   []string
	entity interface{}
}

type PolicyStorageTransaction struct {
	tag      uuid.UUID
	attrs    map[string]Attribute
	policies Evaluable
	err      error
}

func (t *PolicyStorageTransaction) Attributes() map[string]Attribute {
	return t.attrs
}

func (t *PolicyStorageTransaction) applyCmd(cmd *command) error {
	switch cmd.op {
	case UOAdd:
		return t.appendItem(cmd.path, cmd.entity)

	case UODelete:
		return t.del(cmd.path)
	}

	return newUnknownPolicyUpdateOperationError(cmd.op)
}

func (t *PolicyStorageTransaction) Apply(u *PolicyUpdate) error {
	if t.err != nil {
		return newFailedPolicyTransactionError(t.tag, t.err)
	}

	if t.tag.String() != u.oldTag.String() {
		return newPolicyTransactionTagsNotMatchError(t.tag, u.oldTag)
	}

	for i, cmd := range u.cmds {
		err := t.applyCmd(cmd)
		if err != nil {
			t.err = err
			return bindErrorf(err, "command %d", i)
		}
	}

	t.tag = u.newTag
	return nil
}

func (t *PolicyStorageTransaction) Commit() (*PolicyStorage, error) {
	if t.err != nil {
		return nil, newFailedPolicyTransactionError(t.tag, t.err)
	}

	return &PolicyStorage{tag: &t.tag, attrs: t.attrs, policies: t.policies}, nil
}

func (t *PolicyStorageTransaction) appendItem(path []string, v interface{}) error {
	if len(path) <= 0 {
		p, ok := v.(Evaluable)
		if !ok {
			return newInvalidRootPolicyItemTypeError(v)
		}

		if _, ok := p.GetID(); !ok {
			return newHiddenRootPolicyAppendError()
		}

		t.policies = p
		return nil
	}

	ID := path[0]

	if pID, ok := t.policies.GetID(); ok && pID != ID {
		return newInvalidRootPolicyError(ID, pID)
	}

	p, err := t.policies.Append(path[1:], v)
	if err != nil {
		return err
	}

	t.policies = p
	return nil
}

func (t *PolicyStorageTransaction) del(path []string) error {
	if len(path) <= 0 {
		return newEmptyPathModificationError()
	}

	ID := path[0]

	if pID, ok := t.policies.GetID(); ok && pID != ID {
		return newInvalidRootPolicyError(ID, pID)
	}

	if len(path) > 1 {
		p, err := t.policies.Delete(path[1:])
		if err != nil {
			return err
		}

		t.policies = p
		return nil
	}

	t.policies = nil
	return nil
}
