package libyear

import (
	"path/filepath"

	"github.com/nieomylnieja/go-libyear/internal"
)

func NewCommandBuilder(source Source, output Output) CommandBuilder {
	return CommandBuilder{
		source: source,
		output: output,
	}
}

type CommandBuilder struct {
	source        Source
	output        Output
	repo          ModulesRepo
	fallback      VersionsGetter
	withCache     bool
	cacheFilePath string
	opts          Option
	goprivate     string
	vcsRegistry   *VCSRegistry
}

func (b CommandBuilder) WithCache(cacheFilePath string) CommandBuilder {
	b.withCache = true
	b.cacheFilePath = cacheFilePath
	return b
}

func (b CommandBuilder) WithModulesRepo(repo ModulesRepo) CommandBuilder {
	b.repo = repo
	return b
}

func (b CommandBuilder) WithFallbackVersionsGetter(getter VersionsGetter) CommandBuilder {
	b.fallback = getter
	return b
}

func (b CommandBuilder) WithOptions(opts ...Option) CommandBuilder {
	for _, opt := range opts {
		b.opts |= opt
	}
	return b
}

func (b CommandBuilder) WithGOPRIVATE(pattern string) CommandBuilder {
	b.goprivate = pattern
	return b
}

func (b CommandBuilder) WithVCSRegistry(registry *VCSRegistry) CommandBuilder {
	b.vcsRegistry = registry
	return b
}

func (b CommandBuilder) Build() (*Command, error) {
	if b.repo == nil {
		var err error
		if b.opts&OptionUseGoList != 0 {
			b.repo, err = internal.NewGoListExecutor(b.withCache, b.cacheFilePath)
		} else {
			b.repo, err = internal.NewGoProxyClient(b.withCache, b.cacheFilePath)
		}
		if err != nil {
			return nil, err
		}
	}
	if b.fallback == nil {
		b.fallback = internal.NewDepsDevClient()
	}
	// Share initialized ModulesRepo with sources.
	if v, ok := b.source.(interface{ SetModulesRepo(repo ModulesRepo) }); ok {
		v.SetModulesRepo(b.repo)
	}
	if b.vcsRegistry == nil {
		cacheBase, err := internal.GetDefaultCacheBasePath()
		if err != nil {
			return nil, err
		}
		cacheDir := filepath.Join(cacheBase, "vcs")
		b.vcsRegistry = NewVCSRegistry(cacheDir)
	}
	return &Command{
		source:           b.source,
		output:           b.output,
		repo:             b.repo,
		fallbackVersions: b.fallback,
		opts:             b.opts,
		goprivate:        b.goprivate,
		vcs:              b.vcsRegistry,
	}, nil
}
