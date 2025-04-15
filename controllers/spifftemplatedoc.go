package controllers

import (
	"bytes"
	"container/list"
	"fmt"
	"math"

	"github.com/mikefarah/yq/v4/pkg/yqlib"
	"ocm.software/ocm/api/ocm/ocmutils/localize"
	ocmruntime "ocm.software/ocm/api/utils/runtime"
)

// Key we store spiff rules under within spiff template doc.
const ocmAdjustmentsTemplateKey = "ocmAdjustmentsTemplateKey"

// makeYqNode unmarshalls bytes into a CandidateNode
// In order to make debugging easier later during yaml processing we take in
// file name, document index and file index values which are then set on
// the CandidateNode.
func makeYqNode(docBytes []byte, fileName string, docIndex, fileIndex uint) (*yqlib.CandidateNode, error) {
	yamlPreferences := yqlib.NewDefaultYamlPreferences()
	yamlDecoder := yqlib.NewYamlDecoder(yamlPreferences)
	rdr := bytes.NewBuffer(docBytes)

	if err := yamlDecoder.Init(rdr); err != nil {
		return nil, err
	}

	ret, err := yamlDecoder.Decode()
	if err != nil {
		return nil, err
	}

	ret.SetDocument(docIndex)
	ret.SetFilename(fileName)
	if fileIndex > math.MaxInt {
		return nil, fmt.Errorf("file index value would cause integer overflow: %d", fileIndex)
	}
	ret.SetFileIndex(int(fileIndex))

	return ret, nil
}

// spiffTemplateDoc is a wrapper around CandidateNode.  Its purpose is to make the code in
// MutationReconcileLooper::generateSubstitutions easier to read.
//
// spiffTemplateDoc also represents a spiff++ template with a particular structure :
// ( defaults yaml merged with config values yaml )
// + sequence of ocm controller conifg rules stored under the key 'ocmAdjustmentsTemplateKey'.
type spiffTemplateDoc yqlib.CandidateNode

// Store subst, in json form, under key ocmAdjustmentsTemplateKey.
func (s *spiffTemplateDoc) addSpiffRules(subst []localize.Substitution) error {
	var err error

	var adjustmentBytes []byte
	if adjustmentBytes, err = ocmruntime.DefaultJSONEncoding.Marshal(subst); err != nil {
		return fmt.Errorf("failed to marshal substitutions: %w", err)
	}

	var adjustmentsDoc *yqlib.CandidateNode

	const fileIndex = 2
	if adjustmentsDoc, err = makeYqNode(adjustmentBytes, "adjustments", 0, fileIndex); err != nil {
		return fmt.Errorf("failed to marshal adjustments: %w", err)
	}

	((*yqlib.CandidateNode)(s)).AddKeyValueChild(
		&yqlib.CandidateNode{
			Kind:  yqlib.ScalarNode,
			Value: ocmAdjustmentsTemplateKey,
		},
		adjustmentsDoc)

	return nil
}

func (s *spiffTemplateDoc) marshal() ([]byte, error) {
	encoder := yqlib.NewYamlEncoder(yqlib.NewDefaultYamlPreferences())
	buf := bytes.NewBuffer([]byte{})
	pw := yqlib.NewSinglePrinterWriter(buf)
	p := yqlib.NewPrinter(encoder, pw)

	// Often yq is working with list of nodes because matches what
	// yaml files are.  Yaml files are sequences of yaml documents.
	// This is one of those case.
	nodes := list.New()
	nodes.PushBack(((*yqlib.CandidateNode)(s)))

	if err := p.PrintResults(nodes); err != nil {
		return nil, fmt.Errorf("failed to print results: %w", err)
	}

	return buf.Bytes(), nil
}

// mergeDefaultsAndConfigValues unmarshals defaults and configValues to yaml
// and then returns the deep merge result produced by yqlib.
func mergeDefaultsAndConfigValues(defaults, configValues []byte) (*spiffTemplateDoc, error) {
	var err error
	var defaultsDoc, configValuesDoc *yqlib.CandidateNode

	if defaultsDoc, err = makeYqNode(defaults, "defaults", 0, 0); err != nil {
		return nil, fmt.Errorf("failed to make default document: %w", err)
	}

	if configValuesDoc, err = makeYqNode(configValues, "values", 0, 1); err != nil {
		return nil, fmt.Errorf("failed to parse values values: %w", err)
	}

	evaluator := yqlib.NewAllAtOnceEvaluator()
	var mergeResult *list.List
	const mergeExpression = ".  as $item ireduce({}; . * $item )"

	// We get back a list because yq supports expressions that produce
	// multiple yaml documents.
	// And unfortunately golang lists aren't type safe.
	// So now we have all these assertions to make sure we got back a single
	// yaml document like we expect.
	if mergeResult, err = evaluator.EvaluateNodes(mergeExpression, defaultsDoc, configValuesDoc); err != nil {
		return nil, fmt.Errorf("failed to evaluate nodes: %w", err)
	}

	if docCount := mergeResult.Len(); docCount != 1 {
		return nil, fmt.Errorf("merge of defaults and config has incorrect doc count. docCount: %v expression: '%v' defaults: '%v' config values '%v",
			docCount, mergeExpression, defaultsDoc, configValuesDoc)
	}

	if value := mergeResult.Front().Value; value == nil {
		return nil, fmt.Errorf("merge of defaults and config values produced nil result. expression: '%v' defaults: '%v' config values '%v",
			mergeExpression, defaultsDoc, configValuesDoc)
	} else if node, ok := value.(*yqlib.CandidateNode); ok {
		return ((*spiffTemplateDoc)(node)), nil
	}

	return nil, fmt.Errorf("merge of defaults and config values did not produce *CandidateNode. expression: '%v' defaults: '%v' config values '%v",
		mergeExpression, defaultsDoc, configValuesDoc)
}
