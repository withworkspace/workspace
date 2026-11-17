; TypeScript (and JavaScript, with the TSX grammar): what code navigation
; reads of a file.

; Scopes

(statement_block) @scope.block
(function_declaration) @scope.function
(generator_function_declaration) @scope.function
(function_expression) @scope.function
(arrow_function) @scope.function
(method_definition) @scope.function
(class_declaration) @scope.class
(abstract_class_declaration) @scope.class
(class) @scope.class
(interface_declaration) @scope.class
(for_statement) @scope.block
(for_in_statement) @scope.block
(catch_clause) @scope.block
(internal_module name: (_) @namespace.name) @scope.namespace

; Imports

(import_statement
  (import_clause (identifier) @import.default)
  source: (string (string_fragment) @import.path))
(import_statement
  (import_clause (namespace_import (identifier) @import.alias))
  source: (string (string_fragment) @import.path @import.namespace))
(import_statement
  (import_clause (named_imports (import_specifier name: (_) @import.named alias: (_)? @import.alias)))
  source: (string (string_fragment) @import.path))
; Every import's path: one that brings in nothing (import "./side") is
; only this.
(import_statement
  source: (string (string_fragment) @import.path @import.blank))

(variable_declarator
  name: (identifier) @import.alias
  value: (call_expression
    function: (identifier) @_require (#eq? @_require "require")
    arguments: (arguments . (string (string_fragment) @import.path @import.module) .)))
(variable_declarator
  name: (object_pattern (shorthand_property_identifier_pattern) @import.named)
  value: (call_expression
    function: (identifier) @_require (#eq? @_require "require")
    arguments: (arguments . (string (string_fragment) @import.path) .)))

; What's exported: from another module, or by another name.

(export_statement
  (export_clause (export_specifier name: (_) @reexport.named alias: (_)? @import.alias))
  source: (string (string_fragment) @import.path))
(export_statement
  "*"
  source: (string (string_fragment) @import.path @reexport.star))
(export_statement
  (namespace_export (_) @import.alias)
  source: (string (string_fragment) @import.path @reexport.namespace))
(export_statement
  (export_clause (export_specifier name: (_) @reexport.named alias: (_)? @import.alias))
  !source)
(export_statement "default" value: (identifier) @reexport.default)

(export_statement declaration: (_ name: (_) @export.name))
(export_statement declaration: (lexical_declaration (variable_declarator name: (identifier) @export.name)))
(export_statement declaration: (variable_declaration (variable_declarator name: (identifier) @export.name)))
(export_statement "default" declaration: (_ name: (_) @export.default))

; Functions, classes, interfaces, types, enums, namespaces

(function_declaration
  name: (identifier) @def.func
  return_type: (type_annotation (_) @def.typed)?) @def.extent
(generator_function_declaration name: (identifier) @def.func) @def.extent
(function_signature
  name: (identifier) @def.func
  return_type: (type_annotation (_) @def.typed)?) @def.extent

(class_declaration name: (_) @def.class) @def.extent
(abstract_class_declaration name: (_) @def.class) @def.extent
(class_declaration name: (_) @def.class (class_heritage (extends_clause value: (_) @def.base)))
(class_declaration name: (_) @def.class (class_heritage (implements_clause (_) @def.base)))
(abstract_class_declaration name: (_) @def.class (class_heritage (extends_clause value: (_) @def.base)))

(interface_declaration name: (_) @def.interface) @def.extent
(interface_declaration name: (_) @def.interface (extends_type_clause type: (_) @def.base))

(type_alias_declaration name: (_) @def.type) @def.extent
(enum_declaration name: (_) @def.enum) @def.extent
(enum_body name: (property_identifier) @def.member)
(enum_assignment name: (property_identifier) @def.member) @def.extent
(internal_module name: (identifier) @def.namespace) @def.extent
(type_parameter name: (type_identifier) @def.typeparam)

; Members

(method_definition
  name: (property_identifier) @def.method
  return_type: (type_annotation (_) @def.typed)?) @def.extent
(public_field_definition
  name: (property_identifier) @def.field
  type: (type_annotation (_) @def.typed)?) @def.extent
(public_field_definition
  name: (property_identifier) @def.field
  value: (new_expression constructor: (_) @def.new))
(method_signature
  name: (property_identifier) @def.method
  return_type: (type_annotation (_) @def.typed)?) @def.extent
(property_signature
  name: (property_identifier) @def.field
  type: (type_annotation (_) @def.typed)?) @def.extent
(abstract_method_signature
  name: (property_identifier) @def.method
  return_type: (type_annotation (_) @def.typed)?) @def.extent

; A constructor's parameter with private, public or readonly is a field.
(method_definition
  name: (property_identifier) @_constructor (#eq? @_constructor "constructor")
  parameters: (formal_parameters
    (required_parameter
      (accessibility_modifier)
      pattern: (identifier) @def.field
      type: (type_annotation (_) @def.typed)?)))

; Variables and parameters

(variable_declarator
  name: (identifier) @def.var
  type: (type_annotation (_) @def.typed)?) @def.extent
(variable_declarator
  name: (identifier) @def.var
  value: (new_expression constructor: (_) @def.new))
(variable_declarator
  name: (identifier) @def.var
  value: (call_expression function: (_) @def.from))
(variable_declarator
  name: (identifier) @def.var
  value: (await_expression (call_expression function: (_) @def.from)))
(variable_declarator
  name: (identifier) @def.func
  value: [(arrow_function) (function_expression)] @def.extent)
(variable_declarator
  name: (identifier) @def.func
  value: [(arrow_function return_type: (type_annotation (_) @def.typed)) (function_expression return_type: (type_annotation (_) @def.typed))])

(object_pattern (shorthand_property_identifier_pattern) @def.var)
(object_pattern (pair_pattern value: (identifier) @def.var))
(object_pattern (object_assignment_pattern left: (shorthand_property_identifier_pattern) @def.var))
(array_pattern (identifier) @def.var)
(rest_pattern (identifier) @def.var)

(required_parameter
  pattern: (identifier) @def.param
  type: (type_annotation (_) @def.typed)?)
(optional_parameter
  pattern: (identifier) @def.param
  type: (type_annotation (_) @def.typed)?)
(arrow_function parameter: (identifier) @def.param)
(for_in_statement left: (identifier) @def.var)
(for_in_statement left: (identifier) @def.var "of" right: (_) @def.elem)
(catch_clause parameter: (identifier) @def.var)

; Uses

(identifier) @ref
(type_identifier) @ref.type
(shorthand_property_identifier) @ref
(call_expression function: (identifier) @ref.call)
(new_expression constructor: (identifier) @ref.call)

(member_expression
  object: (_) @member.object
  property: (property_identifier) @member)
(call_expression
  function: (member_expression
    object: (_) @member.object
    property: (property_identifier) @member.call))
(nested_type_identifier
  module: (_) @member.object
  name: (type_identifier) @member.type)
(nested_identifier
  object: (_) @member.object
  property: (_) @member)

(call_expression function: (_) @call.callee) @call
(new_expression constructor: (_) @call.callee) @call
