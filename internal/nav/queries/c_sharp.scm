; C#: what code navigation reads of a file. A type is found by its name
; and kept if its namespace is in reach (csharp.go), so namespaces and
; usings matter more than imports do elsewhere.

; Namespaces

(namespace_declaration name: (_) @namespace.name @skip.within) @scope.namespace
(file_scoped_namespace_declaration name: (_) @namespace.file @skip.within)

; Usings: a namespace, a type's statics, an alias; global for the whole
; project. The most telling of a using's matches is the one kept.

(using_directive !name [(identifier) (qualified_name)] @import.path @import.using)
(using_directive "static" [(identifier) (qualified_name) (generic_name)] @import.path @import.static)
(using_directive name: (identifier) @import.alias [(identifier) (qualified_name) (generic_name)] @import.path @import.typealias)
(using_directive "global" [(identifier) (qualified_name)] @import.path @import.global)

; Scopes

(class_declaration) @scope.class
(struct_declaration) @scope.class
(interface_declaration) @scope.class
(record_declaration) @scope.class
(enum_declaration) @scope.class
(method_declaration) @scope.function
(constructor_declaration) @scope.function
(local_function_statement) @scope.function
(lambda_expression) @scope.function
(accessor_declaration) @scope.function
(block) @scope.block
(for_statement) @scope.block
(foreach_statement) @scope.block
(using_statement) @scope.block
(catch_clause) @scope.block
(switch_section) @scope.block

; Types, with their bases

(class_declaration name: (identifier) @def.class) @def.extent
(class_declaration name: (identifier) @def.class (base_list (_) @def.base))
(struct_declaration name: (identifier) @def.struct) @def.extent
(struct_declaration name: (identifier) @def.struct (base_list (_) @def.base))
(interface_declaration name: (identifier) @def.interface) @def.extent
(interface_declaration name: (identifier) @def.interface (base_list (_) @def.base))
(record_declaration name: (identifier) @def.class) @def.extent
(record_declaration name: (identifier) @def.class (base_list (_) @def.base))
(enum_declaration name: (identifier) @def.enum) @def.extent
(delegate_declaration name: (identifier) @def.type) @def.extent
(enum_member_declaration name: (identifier) @def.member) @def.extent
(type_parameter name: (identifier) @def.typeparam)

; Members

(method_declaration returns: (_) @def.typed name: (identifier) @def.method) @def.extent
(constructor_declaration name: (identifier) @def.method) @def.extent
(field_declaration (variable_declaration type: (_) @def.typed (variable_declarator name: (identifier) @def.field))) @def.extent
(event_field_declaration (variable_declaration type: (_) @def.typed (variable_declarator name: (identifier) @def.field))) @def.extent
(property_declaration type: (_) @def.typed name: (identifier) @def.prop) @def.extent
(record_declaration (parameter_list (parameter type: (_) @def.typed name: (identifier) @def.prop)))
(local_function_statement type: (_) @def.typed name: (identifier) @def.func) @def.extent

; Variables and parameters

(local_declaration_statement (variable_declaration type: (_) @def.typed (variable_declarator name: (identifier) @def.var))) @def.extent
(variable_declarator name: (identifier) @def.var (object_creation_expression type: (_) @def.new))
(variable_declarator name: (identifier) @def.var (invocation_expression function: (_) @def.from))
(variable_declarator name: (identifier) @def.var (await_expression (invocation_expression function: (_) @def.from)))
(variable_declarator name: (identifier) @def.var [(identifier) (member_access_expression)] @def.from)
(using_statement (variable_declaration type: (_) @def.typed (variable_declarator name: (identifier) @def.var)))
(for_statement (variable_declaration type: (_) @def.typed (variable_declarator name: (identifier) @def.var)))
(parameter type: (_)? @def.typed name: (identifier) @def.param)
(lambda_expression parameters: (implicit_parameter) @def.param)
(foreach_statement type: (_) @def.typed left: (identifier) @def.var)
(foreach_statement type: (implicit_type) left: (identifier) @def.var right: (_) @def.elem)
(catch_declaration type: (_) @def.typed name: (identifier) @def.var)
(declaration_expression type: (_) @def.typed name: (identifier) @def.var)
(declaration_pattern type: (_) @def.typed name: (identifier) @def.var)

; Uses

(alias_qualified_name alias: (identifier) @skip)
(argument name: (identifier) @skip)

(identifier) @ref
(invocation_expression function: (identifier) @ref.call)
(object_creation_expression type: (identifier) @ref.call)
(variable_declaration type: (identifier) @ref.type)
(parameter type: (identifier) @ref.type)
(base_list (identifier) @ref.type)
(generic_name (identifier) @ref.type)

(member_access_expression expression: (_) @member.object name: (identifier) @member)
(member_access_expression expression: (_) @member.object name: (generic_name (identifier) @member))
(invocation_expression function: (member_access_expression expression: (_) @member.object name: (identifier) @member.call))
(invocation_expression function: (member_access_expression expression: (_) @member.object name: (generic_name (identifier) @member.call)))
(member_access_expression "this" @member.object name: (identifier) @member)
(member_access_expression "base" @member.object name: (identifier) @member)
(invocation_expression function: (member_access_expression "this" @member.object name: (identifier) @member.call))
(invocation_expression function: (member_access_expression "base" @member.object name: (identifier) @member.call))
(conditional_access_expression condition: (_) @member.object (member_binding_expression name: (identifier) @member))
(qualified_name qualifier: (_) @member.object name: (identifier) @member.type)

(invocation_expression function: (_) @call.callee) @call
(object_creation_expression type: (_) @call.callee) @call
