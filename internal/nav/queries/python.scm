; Python: what code navigation reads of a file.

; Scopes. A class's body is one, but its methods don't see into it
; (python.go says so).

(function_definition) @scope.function
(lambda) @scope.function
(class_definition) @scope.class
(list_comprehension) @scope.function
(set_comprehension) @scope.function
(dictionary_comprehension) @scope.function
(generator_expression) @scope.function

; Imports

(import_statement name: (dotted_name) @import.path @import.module)
(import_statement
  name: (aliased_import name: (dotted_name) @import.path @import.module alias: (identifier) @import.alias))
(import_from_statement
  module_name: (_) @import.path
  name: (dotted_name (identifier) @import.named))
(import_from_statement
  module_name: (_) @import.path
  name: (aliased_import name: (dotted_name (identifier) @import.named) alias: (identifier) @import.alias))
(import_from_statement
  module_name: (_) @import.path
  (wildcard_import) @import.star)

; Functions and classes, with their docstrings

(function_definition
  name: (identifier) @def.func
  return_type: (type)? @def.typed) @def.extent
(function_definition
  name: (identifier) @def.func
  body: (block . (expression_statement (string) @def.doc)))
(class_definition name: (identifier) @def.class) @def.extent
(class_definition
  name: (identifier) @def.class
  superclasses: (argument_list [(identifier) (attribute)] @def.base))
(class_definition
  name: (identifier) @def.class
  body: (block . (expression_statement (string) @def.doc)))

; global x, nonlocal x: assigning x in this function is to the x outside.
(global_statement (identifier) @global.name)
(nonlocal_statement (identifier) @global.name)

; The names in a module's path are the path, not names of their own.
(import_statement name: (dotted_name (identifier) @skip))
(import_statement name: (aliased_import name: (dotted_name (identifier) @skip)))
(import_from_statement module_name: (dotted_name (identifier) @skip))
(import_from_statement module_name: (relative_import (dotted_name (identifier) @skip)))

; from __future__ import annotations names no definition.
(future_import_statement name: (dotted_name (identifier) @skip))

; Variables: assigned, annotated, in a for, a with, an except, a walrus

(assignment left: (identifier) @def.var type: (type)? @def.typed) @def.extent
(assignment left: (identifier) @def.var right: (call function: (_) @def.from))
(assignment left: (identifier) @def.var right: [(identifier) (attribute)] @def.from)
(assignment left: (pattern_list (identifier) @def.var))
(assignment left: (tuple_pattern (identifier) @def.var))
(augmented_assignment left: (identifier) @def.var)
(for_statement left: (identifier) @def.var)
(for_statement left: (identifier) @def.var right: (_) @def.elem)
(for_statement left: (pattern_list (identifier) @def.var))
(for_statement left: (pattern_list [(tuple_pattern (identifier) @def.var) (list_pattern (identifier) @def.var)]))
(for_statement left: (tuple_pattern (identifier) @def.var))
(for_in_clause left: (identifier) @def.var)
(for_in_clause left: (pattern_list (identifier) @def.var))
(for_in_clause left: (pattern_list [(tuple_pattern (identifier) @def.var) (list_pattern (identifier) @def.var)]))
(for_in_clause left: (tuple_pattern (identifier) @def.var))
(assignment left: (pattern_list [(tuple_pattern (identifier) @def.var) (list_pattern (identifier) @def.var)]))
(as_pattern_target (identifier) @def.var)
(named_expression name: (identifier) @def.var)

; self.x = … in a method is the class's field x.
(assignment
  left: (attribute object: (identifier) @_self (#eq? @_self "self") attribute: (identifier) @def.field)
  type: (type)? @def.typed) @def.extent
(assignment
  left: (attribute object: (identifier) @_self (#eq? @_self "self") attribute: (identifier) @def.field)
  right: [(identifier) (attribute) (call function: (_))] @def.from)

; Parameters

(parameters (identifier) @def.param)
(parameters (typed_parameter (identifier) @def.param type: (type) @def.typed))
(parameters (default_parameter name: (identifier) @def.param))
(parameters (typed_default_parameter name: (identifier) @def.param type: (type) @def.typed))
(parameters (list_splat_pattern (identifier) @def.param))
(parameters (dictionary_splat_pattern (identifier) @def.param))
(parameters (typed_parameter (list_splat_pattern (identifier) @def.param)))
(parameters (typed_parameter (dictionary_splat_pattern (identifier) @def.param)))
(lambda_parameters (identifier) @def.param)

; Uses

(identifier) @ref
(call function: (identifier) @ref.call)
(type (identifier) @ref.type)
(attribute object: (_) @member.object attribute: (identifier) @member)
(call function: (attribute object: (_) @member.object attribute: (identifier) @member.call))
(keyword_argument name: (identifier) @skip)

(call function: (_) @call.callee) @call
