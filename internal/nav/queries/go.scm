; Go: what code navigation reads of a file.

(package_clause (package_identifier) @package)

; Scopes

(function_declaration) @scope.function
(method_declaration) @scope.function
(func_literal) @scope.function
(type_spec type_parameters: (_)) @scope.block
(block) @scope.block
(for_statement) @scope.block
(if_statement) @scope.block
(expression_switch_statement) @scope.block
(type_switch_statement) @scope.block
(select_statement) @scope.block
(expression_case) @scope.block
(type_case) @scope.block
(default_case) @scope.block
(communication_case) @scope.block

; Imports

(import_spec
  name: (package_identifier) @import.alias
  path: (_ (interpreted_string_literal_content) @import.path @import.module))
(import_spec
  !name
  path: (_ (interpreted_string_literal_content) @import.path @import.module))
(import_spec
  name: (dot)
  path: (_ (interpreted_string_literal_content) @import.path @import.star))
(import_spec
  name: (blank_identifier)
  path: (_ (interpreted_string_literal_content) @import.path @import.blank))

; Functions and methods

(function_declaration
  name: (identifier) @def.func
  result: (_)? @def.typed) @def.extent

(method_declaration
  receiver: (parameter_list (parameter_declaration type: (_) @def.receiver))
  name: (field_identifier) @def.method
  result: (_)? @def.typed) @def.extent

(parameter_declaration name: (identifier) @def.param type: (_) @def.typed)
(variadic_parameter_declaration name: (identifier) @def.param type: (_) @def.typed)
(type_parameter_declaration name: (identifier) @def.typeparam)

; Types

(type_spec name: (type_identifier) @def.type) @def.extent
(type_alias name: (type_identifier) @def.type) @def.extent
(type_spec name: (type_identifier) @def.struct type: (struct_type)) @def.extent
(type_spec name: (type_identifier) @def.interface type: (interface_type)) @def.extent

; An embedded field is a base: its fields and methods are the struct's.
(type_spec
  name: (type_identifier) @def.struct
  type: (struct_type (field_declaration_list (field_declaration !name type: (_) @def.base))))
(type_spec
  name: (type_identifier) @def.interface
  type: (interface_type (type_elem . (_) @def.base .)))

(field_declaration name: (field_identifier) @def.field type: (_) @def.typed) @def.extent
(method_elem name: (field_identifier) @def.method result: (_)? @def.typed) @def.extent

; Variables and constants

(var_spec name: (identifier) @def.var type: (_)? @def.typed) @def.extent
(const_spec name: (identifier) @def.const type: (_)? @def.typed) @def.extent
(var_spec
  name: (identifier) @def.var .
  value: (expression_list . (call_expression function: (_) @def.from)))
(var_spec
  name: (identifier) @def.var .
  value: (expression_list . [
    (composite_literal type: (_) @def.new)
    (unary_expression operand: (composite_literal type: (_) @def.new))
  ]))

(short_var_declaration left: (expression_list (identifier) @def.var)) @def.extent
(short_var_declaration
  left: (expression_list . (identifier) @def.var)
  right: (expression_list . (call_expression function: (_) @def.from)))
(short_var_declaration
  left: (expression_list . (identifier) @def.var)
  right: (expression_list . [
    (composite_literal type: (_) @def.new)
    (unary_expression operand: (composite_literal type: (_) @def.new))
  ]))

(range_clause left: (expression_list (identifier) @def.var))
(receive_statement left: (expression_list (identifier) @def.var))
(type_switch_statement alias: (expression_list (identifier) @def.var))

; Uses

(identifier) @ref
(type_identifier) @ref.type
(package_identifier) @ref
(call_expression function: (identifier) @ref.call)

(selector_expression
  operand: (_) @member.object
  field: (field_identifier) @member)
(call_expression
  function: (selector_expression
    operand: (_) @member.object
    field: (field_identifier) @member.call))
(qualified_type
  package: (_) @member.object
  name: (type_identifier) @member.type)

; A field named in a struct's literal: Server{name: …}.
(composite_literal
  type: [(type_identifier) (qualified_type) (generic_type)] @member.object
  body: (literal_value (keyed_element key: (literal_element (identifier) @member))))

(call_expression function: (_) @call.callee) @call
