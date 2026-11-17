; TypeSpec: what code navigation reads of a file. As C#: a name is found
; by its namespaces in reach (typespec.go), and imports name files.

; Namespaces: a block's is a scope; one without a block is the file's.

(namespace_statement name: (_) @namespace.file @skip.within)
(namespace_statement name: (_) @namespace.name (namespace_body)) @scope.namespace

; Imports and usings

(import_statement (quoted_string_literal (quoted_string_fragment) @import.path @import.module))
(using_statement module: (_) @import.path @import.using)

; Scopes

(model_statement) @scope.class
(interface_statement) @scope.class
(enum_statement) @scope.class
(union_statement) @scope.class
(operation_statement) @scope.function
(interface_member) @scope.function
(decorator_declaration_statement) @scope.function

; Declarations, with what they're built on

(model_statement name: (identifier) @def.class) @def.extent
(model_statement name: (identifier) @def.class (model_extends_heritage (reference_expression) @def.base))
(model_statement name: (identifier) @def.class (model_is_heritage (reference_expression) @def.base))
(model_statement name: (identifier) @def.class
  (model_expression (model_body (model_property (model_spread_property (reference_expression) @def.base)))))
(scalar_statement name: (identifier) @def.type) @def.extent
(scalar_statement name: (identifier) @def.type (scalar_extends (reference_expression) @def.base))
(enum_statement name: (identifier) @def.enum) @def.extent
(enum_member name: (identifier) @def.member) @def.extent
(union_statement name: (identifier) @def.enum) @def.extent
(union_variant name: (identifier) @def.member (reference_expression)? @def.typed) @def.extent
(interface_statement name: (identifier) @def.interface) @def.extent
(interface_member
  name: (identifier) @def.method
  (operation_signature_declaration (reference_expression) @def.typed)?) @def.extent
(operation_statement
  name: (identifier) @def.func
  (operation_signature_declaration (reference_expression) @def.typed)?) @def.extent
(alias_statement name: (identifier) @def.type (reference_expression) @def.typed) @def.extent
(const_statement name: (identifier) @def.const) @def.extent
(decorator_declaration_statement name: (identifier) @def.func) @def.extent
(template_parameter name: (identifier) @def.typeparam)
(model_property name: (identifier) @def.field type: (_) @def.typed) @def.extent
(function_parameter name: (identifier) @def.param)

; Uses: a name, a builtin type, a decorator; A.B.C as one node whose
; members' objects are worked out from where they are.

(identifier) @ref
(builtin_type) @ref.type
(decorator name: (identifier_or_member_expression (identifier) @ref.call))
(member_expression base: (_) @chain.base member: (identifier)+ @chain.member)

; An object's key isn't a name.
(object_member key: (identifier) @skip)
