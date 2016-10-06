# Exclude vendor folder from project subfolders
SUBDIRS = $(filter-out vendor/., $(wildcard */.))
SUBCLEAN = $(addsuffix .clean,$(SUBDIRS))
SUBTEST = $(addsuffix .test,$(SUBDIRS))
SUBLINT = $(addsuffix .lint,$(SUBDIRS))
SUBINSTALL = $(addsuffix .install,$(SUBDIRS))
SUBBUILD = $(addsuffix .build,$(SUBDIRS))

# invoke make all for all subprojects
all: $(SUBDIRS)

$(SUBDIRS):
	$(MAKE) -C $@

# invoke make clean for all subprojects
clean: $(SUBCLEAN)

$(SUBCLEAN): %.clean:
	$(MAKE) -C $* clean

# invoke make test for all subprojects
test: $(SUBTEST)

$(SUBTEST): %.test:
	$(MAKE) -C $* test

# invoke make lint for all subprojects
lint: $(SUBLINT)

$(SUBLINT): %.lint:
	$(MAKE) -C $* lint

install: $(SUBINSTALL)

$(SUBINSTALL): %.install:
	$(MAKE) -C $* install

build: $(SUBBUILD)

$(SUBBUILD): %.build:
	$(MAKE) -C $* build

.PHONY: all $(SUBDIRS) clean $(SUBCLEAN) test $(SUBTEST) lint $(SUBLINT) \
	install $(SUBINSTALL) build $(SUBBUILD)

