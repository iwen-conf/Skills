package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

type businessValue struct {
	kind   string
	scalar any
	items  []businessValue
}

type businessAttributeInfo struct {
	subject   BusinessSubject
	attribute BusinessAttribute
}

func businessAttributeIndex(contract *BusinessContract) map[string]businessAttributeInfo {
	attributes := map[string]businessAttributeInfo{}
	if contract != nil {
		for _, subject := range contract.Subjects {
			for _, attribute := range subject.Attributes {
				attributes[subject.ID+"."+attribute.Name] = businessAttributeInfo{subject, attribute}
			}
		}
	}
	return attributes
}

func businessKnownType(kind string) bool {
	switch kind {
	case "boolean", "string", "integer", "number", "string_set", "integer_set", "number_set":
		return true
	}
	return false
}

func businessNumeric(kind string) bool    { return kind == "integer" || kind == "number" }
func businessSetType(kind string) bool    { return strings.HasSuffix(kind, "_set") }
func businessBaseType(kind string) string { return strings.TrimSuffix(kind, "_set") }
func businessCompatible(left, right string) bool {
	if left == right {
		return true
	}
	if businessNumeric(left) && businessNumeric(right) {
		return true
	}
	return businessSetType(left) && businessSetType(right) && businessNumeric(businessBaseType(left)) && businessNumeric(businessBaseType(right))
}

// parseBusinessAttributeValue retains the captured JSON token until an explicit
// transport encoding has been checked. In particular, converting 100.0 or 1e2
// to an integer first would erase the distinction this contract promises.
func parseBusinessAttributeValue(attribute BusinessAttribute, raw json.RawMessage) (businessValue, error) {
	if err := validateBusinessAttributeEncoding(attribute); err != nil {
		return businessValue{}, err
	}
	if attribute.Encoding == "" {
		return parseBusinessValue(attribute.Type, raw)
	}
	// json_i64 allows only JSON whitespace around a token. The minus sign is
	// optional; -0 is legal. Decimal/exponent forms, leading zeros, strings,
	// null, composite values and signed overflow are not an encoded integer.
	token := bytes.Trim(raw, " \t\r\n")
	digits := token
	if len(digits) > 0 && digits[0] == '-' {
		digits = digits[1:]
	}
	if len(digits) == 0 || len(digits) > 19 || len(digits) > 1 && digits[0] == '0' {
		return businessValue{}, fmt.Errorf("expected json_i64 integer token")
	}
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return businessValue{}, fmt.Errorf("expected json_i64 integer token")
		}
	}
	integer, err := strconv.ParseInt(string(token), 10, 64)
	if err != nil {
		return businessValue{}, fmt.Errorf("json_i64 integer token is outside signed 64-bit range")
	}
	return businessValue{kind: attribute.Type, scalar: new(big.Rat).SetInt64(integer)}, nil
}

func parseBusinessValue(kind string, raw json.RawMessage) (businessValue, error) {
	value := businessValue{kind: kind}
	if !businessKnownType(kind) {
		return value, fmt.Errorf("unknown attribute type %q", kind)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return value, fmt.Errorf("value is unknown")
	}
	var decoded any
	if err := decodeUniqueJSON(trimmed, &decoded); err != nil {
		return value, err
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return value, err
	}
	if businessSetType(kind) {
		if _, ok := decoded.([]any); !ok {
			return value, fmt.Errorf("expected %s JSON array", kind)
		}
		var members []json.RawMessage
		if err := json.Unmarshal(trimmed, &members); err != nil {
			return value, err
		}
		if len(members) > 1024 {
			return value, fmt.Errorf("set exceeds 1024 members")
		}
		seen := map[string]bool{}
		for _, member := range members {
			item, err := parseBusinessValue(businessBaseType(kind), member)
			if err != nil {
				return value, err
			}
			key := item.key()
			if seen[key] {
				return value, fmt.Errorf("set contains duplicate member %s", key)
			}
			seen[key] = true
			value.items = append(value.items, item)
		}
		return value, nil
	}
	switch kind {
	case "boolean":
		got, ok := decoded.(bool)
		if !ok {
			return value, fmt.Errorf("expected boolean")
		}
		value.scalar = got
	case "string":
		got, ok := decoded.(string)
		if !ok {
			return value, fmt.Errorf("expected string")
		}
		value.scalar = got
	case "integer", "number":
		got, ok := decoded.(json.Number)
		if !ok {
			return value, fmt.Errorf("expected JSON number")
		}
		if len(got.String()) > 256 {
			return value, fmt.Errorf("number exceeds 256 digits")
		}
		// Bound exponents before big.Rat allocates. Exact decimal comparison is
		// required for money and IDs; conversion through float64 is forbidden.
		lower := strings.ToLower(got.String())
		if index := strings.IndexByte(lower, 'e'); index >= 0 {
			exponent, ok := new(big.Int).SetString(lower[index+1:], 10)
			if !ok || exponent.Cmp(big.NewInt(1000)) > 0 || exponent.Cmp(big.NewInt(-1000)) < 0 {
				return value, fmt.Errorf("number exponent exceeds +/-1000")
			}
		}
		decimal, ok := new(big.Rat).SetString(got.String())
		if !ok || kind == "integer" && !decimal.IsInt() {
			return value, fmt.Errorf("expected %s", kind)
		}
		value.scalar = decimal
	}
	return value, nil
}

func (value businessValue) key() string {
	if businessSetType(value.kind) {
		keys := make([]string, 0, len(value.items))
		for _, item := range value.items {
			keys = append(keys, item.key())
		}
		sort.Strings(keys)
		return strings.Join(keys, "\x00")
	}
	if businessNumeric(value.kind) {
		return value.scalar.(*big.Rat).RatString()
	}
	body, _ := json.Marshal(value.scalar)
	return string(body)
}

func businessValueEqual(left, right businessValue) bool {
	return businessCompatible(left.kind, right.kind) && left.key() == right.key()
}

func businessCompare(left, right businessValue) (int, bool) {
	if businessNumeric(left.kind) && businessNumeric(right.kind) {
		return left.scalar.(*big.Rat).Cmp(right.scalar.(*big.Rat)), true
	}
	if left.kind == "string" && right.kind == "string" {
		return strings.Compare(left.scalar.(string), right.scalar.(string)), true
	}
	return 0, false
}

func businessContains(set, item businessValue) bool {
	for _, member := range set.items {
		if businessValueEqual(member, item) {
			return true
		}
	}
	return false
}

func businessTruth(value bool) TruthEvaluation {
	if value {
		return TruthEvaluation{Value: businessTrue}
	}
	return TruthEvaluation{Value: businessFalse}
}

func unknownBusiness(keys []string, reason string) TruthEvaluation {
	return TruthEvaluation{Value: businessUnknown, Unknown: uniqueSortedBusiness(keys), Reason: reason}
}

func uniqueSortedBusiness(items []string) []string {
	seen := map[string]bool{}
	for _, item := range items {
		seen[item] = true
	}
	out := make([]string, 0, len(seen))
	for item := range seen {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

// EvaluateBusinessPredicate is the same typed, three-valued evaluator used by
// branch selection, expected/actual comparison, finite proofs and planning.
// Invalid model expressions return UNKNOWN here; the model validator separately
// classifies them as MODEL_ERROR, before any executable decision can be READY.
func EvaluateBusinessPredicate(contract *BusinessContract, predicate BusinessPredicate, observed BusinessObservation) TruthEvaluation {
	attributes := businessAttributeIndex(contract)
	count := 0
	if err := validateBusinessPredicate(predicate, attributes, 0, &count); err != nil {
		return unknownBusiness(nil, err.Error())
	}
	return evaluateBusinessPredicate(predicate, observed, attributes)
}

func evaluateBusinessPredicate(predicate BusinessPredicate, observed BusinessObservation, attributes map[string]businessAttributeInfo) TruthEvaluation {
	switch predicate.Op {
	case "true":
		return businessTruth(true)
	case "false":
		return businessTruth(false)
	case "and", "or":
		unknown := []string{}
		for _, arg := range predicate.Args {
			result := evaluateBusinessPredicate(arg, observed, attributes)
			if predicate.Op == "and" && result.Value == businessFalse {
				return businessTruth(false)
			}
			if predicate.Op == "or" && result.Value == businessTrue {
				return businessTruth(true)
			}
			if result.Value == businessUnknown {
				unknown = append(unknown, result.Unknown...)
			}
		}
		if len(unknown) > 0 {
			return unknownBusiness(unknown, "predicate needs independent observations")
		}
		return businessTruth(predicate.Op == "and")
	case "not":
		result := evaluateBusinessPredicate(predicate.Args[0], observed, attributes)
		if result.Value == businessUnknown {
			return result
		}
		return businessTruth(result.Value == businessFalse)
	}
	left, leftUnknown := resolveBusinessOperand(*predicate.Left, observed, attributes)
	right, rightUnknown := resolveBusinessOperand(*predicate.Right, observed, attributes)
	if len(leftUnknown)+len(rightUnknown) > 0 {
		return unknownBusiness(append(leftUnknown, rightUnknown...), "missing, unbound or ill-typed independent observation")
	}
	switch predicate.Op {
	case "eq":
		return businessTruth(businessValueEqual(left, right))
	case "ne":
		return businessTruth(!businessValueEqual(left, right))
	case "lt", "lte", "gt", "gte":
		comparison, _ := businessCompare(left, right)
		switch predicate.Op {
		case "lt":
			return businessTruth(comparison < 0)
		case "lte":
			return businessTruth(comparison <= 0)
		case "gt":
			return businessTruth(comparison > 0)
		default:
			return businessTruth(comparison >= 0)
		}
	case "in":
		return businessTruth(businessContains(right, left))
	case "contains":
		return businessTruth(businessContains(left, right))
	case "subset":
		for _, item := range left.items {
			if !businessContains(right, item) {
				return businessTruth(false)
			}
		}
		return businessTruth(true)
	case "intersects":
		for _, item := range left.items {
			if businessContains(right, item) {
				return businessTruth(true)
			}
		}
		return businessTruth(false)
	case "between":
		lo, _ := businessCompare(left, right.items[0])
		hi, _ := businessCompare(left, right.items[1])
		return businessTruth(lo >= 0 && hi <= 0)
	}
	return unknownBusiness(nil, "unsupported predicate")
}

func resolveBusinessOperand(operand BusinessOperand, observed BusinessObservation, attributes map[string]businessAttributeInfo) (businessValue, []string) {
	if operand.Literal != nil {
		value, _ := parseBusinessValue(operand.Literal.Type, operand.Literal.Value)
		return value, nil
	}
	key := operand.Ref.Key()
	info := attributes[key]
	if info.subject.RequiresBinding && strings.TrimSpace(observed.Bindings[info.subject.ID]) == "" {
		return businessValue{}, []string{"binding:" + info.subject.ID}
	}
	value, err := parseBusinessAttributeValue(info.attribute, observed.Values[key])
	if err != nil {
		return businessValue{}, []string{key}
	}
	return value, nil
}

func businessPredicateRefs(predicate BusinessPredicate) []string {
	refs := []string{}
	for _, operand := range []*BusinessOperand{predicate.Left, predicate.Right} {
		if operand != nil && operand.Ref != nil {
			refs = append(refs, operand.Ref.Key())
		}
	}
	for _, arg := range predicate.Args {
		refs = append(refs, businessPredicateRefs(arg)...)
	}
	return uniqueSortedBusiness(refs)
}
