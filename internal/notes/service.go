package notes

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List(filter Filter) ([]Note, error) {
	return s.repository.ListNotes(filter)
}

func (s *Service) Create(parent, title, content string, tags []string) (Note, error) {
	return s.repository.CreateNote(parent, title, content, NormalizeTags(tags))
}

func (s *Service) Save(id, title, content string, tags []string) (Note, error) {
	return s.repository.SaveNote(id, title, content, NormalizeTags(tags))
}

func (s *Service) Delete(id string) error {
	return s.repository.DeleteNote(id)
}
