# Upbound `ui-schema.yaml` Specification

* Owner: Steven Rathbauer ([@rathpc](https://github.com/rathpc))
* Reviewers: Upbound Maintainers, Crossplane Maintainers
* Status: Accepted, revision 1.1

## Revisions

* 1.1 - Dan Mangum (@hasheddan)
  * Removed references to `version` field in `ui-schema.yaml` required fields.

## Terms

* **Stack**: A Stack is a Crossplane stack-manager managed package. These packages may bundle one or many Crossplane
applications or infrastructure provider controllers.
* **Stacks**: Stacks could refer to more than one Crossplane stack-manager packages or the Stack-Manager system as a whole.

See the [Stacks Design Doc](design-doc-stacks.md) for more details about Crossplane Stacks.

## Objective

This document outlines one specific implementation of the `ui-schema.yaml` file that is meant to serve as a helpful
reference to other potential implementors in the future. This specification is driven by what is currently supported via
the Upbound GUI and meant to be used as the ground work for further UI development using this file.

## Proposed File format

OpenAPI v3

## Supported Fields

### Required Fields

Field | Description
--- | ---
`configSections` | An array of sections to display during the configuration of a resource. Each section should contain a `title` field, `items` field and optionally a `description` field.<br /><br />`title`: A string that labels the section.<br />`description`: A string which optionally provides additional details, instructions or context around the section.<br />`items`: An array of item objects to display within the section. [See below for details on currently supported items](#supportedItems).

### Optional Fields

Field | Description
--- | ---
`initialConfig` | A boolean **or** an object containing two optional fields (`order`, `required`) which instructs the UI to include this resource in the initial configuration step.<br /><br />By default if this field is not included at all it will be assumed as a **false** value and not included as part of the the initial configuration. If it is set to **true** it will be included and required to be configured.<br /><br />If defined as an object, you can give the `order` field a number value and prescribe the order of the overall configuration form flow. Additionally if the type of `initialConfig` is determined to be an object it will be assumed that this configuration should be inlcuded and required during the configuration flow.<br /><br />If you would like this configuration to be included, but optional (_skippable_) during the configuration flow, you must set the `required` field within that object to **false**.

`initialConfig` **Example**:

```yaml
initialConfig: true

# OR

initialConfig:
  order: 2
  required: false
```

-----
<a name="supportedItems"></a>

## Supported Items <small>(_Control Types_)</small>

| Control Type |
| --- |
| [singleInput](#singleInput) |
| [multipleInput](#multipleInput) |
| [secretInput](#secretInput) |
| [singleSelect](#singleSelect) |
| [multipleSelect](#multipleSelect) |
| [checkboxSingle](#checkboxSingle) |
| [checkboxGroup](#checkboxGroup) |
| [radioGroup](#radioGroup) |
| [codeSnippet](#codeSnippet) |

<a name="singleInput"></a>

- ### **singleInput**

    Required Properties
    
    Property | Description
    --- | ---
    `controlType` | **singleInput**
    `name` | A string to describe this input. This value is used to manage this input in the UI and should contain only letters, numbers, dashes or underscores.
    `path` | The JSONPath for this input
    `title` | A string to label this input. This value should be a _"Pretty"_ value to display in the UI.

    Optional Properties

    Property | Description
    --- | ---
    `default` | A string or integer value given to this input on render in the UI.
    `description` | A string to describe this input. This will be displayed directly below the input in the UI.
    `placeholder` | A string to be used as this inputs placeholder value.
    `type` | `hidden`, `integer`, `password`, `readonly`, `string`
    `validation` | An array of validation objects that this input must satisfy to be valid. Each object can also contain a `customError` field which is a string to override the default error message shown when that validation criteria fails.<br /><br />**Supported Validations**:<br />`maximum`: A number to define the upper threshold for an integer input type<br />`maxLength`: A number to define the maximum length of the input<br />`minimum`: A number to define the lower threshold for an integer input type<br />`minLength`: A number to define the minimum length of the input<br />`pattern`: A Regular Expression pattern that the value of the input must match<br />`required`: A boolean to stricly require a value

<a name="multipleInput"></a>

- ### **multipleInput**

    Required Properties
    
    Property | Description
    --- | ---
    `controlType` | **multipleInput**
    `name` | A string to describe this textarea. This value is used to manage this textarea in the UI and should contain only letters, numbers, dashes or underscores.
    `path` | The JSONPath for this textarea
    `title` | A string to label this textarea. This value should be a _"Pretty"_ value to display in the UI.

    Optional Properties

    Property | Description
    --- | ---
    `default` | A string given to this textarea on render in the UI.
    `description` | A string to describe this textarea. This will be displayed directly below the textarea in the UI.
    `placeholder` | A string to be used as this textareas placeholder value.
    `rows` | A number to set the "row height" of this textarea.
    `validation` | An array of validation objects that this textarea must satisfy to be valid. Each object can also contain a `customError` field which is a string to override the default error message shown when that validation criteria fails.<br /><br />**Supported Validations**:<br />`maxLength`: A number to define the maximum length of the input<br />`minLength`: A number to define the minimum length of the input<br />`pattern`: A Regular Expression pattern that the value of the input must match<br />`required`: A boolean to stricly require a value

<a name="secretInput"></a>

- ### **secretInput**

    Required Properties
    
    Property | Description
    --- | ---
    `controlType` | **secretInput**
    `name` | A string to describe this input. This value is used to manage this input in the UI and should contain only letters, numbers, dashes or underscores.
    `path` | The JSONPath for this input
    `secret` | An object containing required information for this secret<br /><br />`namePath`: The JSONPath for the secrets _"name"_ field<br />`namespacePath`: The JSONPath for the secrets _"namespace"_ field<br />`keyPath`: The JSONPath for the secrets _"key"_ field<br />`keyValue`: The value for the secrets data key<br />
    `title` | A string to label this input. This value should be a _"Pretty"_ value to display in the UI.

    Optional Properties

    Property | Description
    --- | ---
    `description` | A string to describe this input. This will be displayed directly below the input in the UI.
    `placeholder` | A string to be used as this inputs placeholder value.
    `validation` | An array of validation objects that this input must satisfy to be valid. Each object can also contain a `customError` field which is a string to override the default error message shown when that validation criteria fails.<br /><br />**Supported Validations**:<br />`maxLength`: A number to define the maximum length of the input<br />`minLength`: A number to define the minimum length of the input<br />`pattern`: A Regular Expression pattern that the value of the input must match<br />`required`: A boolean to stricly require a value

<a name="singleSelect"></a>

- ### **singleSelect**

    Required Properties
    
    Property | Description
    --- | ---
    `controlType` | **singleSelect**
    `default` | An array containing a single value which is a string to be set as the selected value on render in the UI. This value should also appear in the `enum` property.
    `enum` | An array of strings to be used as the dropdown values and labels
    `name` | A string to describe this dropdown. This value is used to manage this dropdown in the UI and should contain only letters, numbers, dashes or underscores.
    `path` | The JSONPath for this dropdown
    `title` | A string to label this dropdown. This value should be a _"Pretty"_ value to display in the UI.

    Optional Properties

    Property | Description
    --- | ---
    `description` | A string to describe this dropdown. This will be displayed directly below the dropdown in the UI.
    `placeholder` | A string to be used as this dropdowns placeholder value.
    `validation` | An array of validation objects that this dropdown must satisfy to be valid. Each object can also contain a `customError` field which is a string to override the default error message shown when that validation criteria fails.<br /><br />**Supported Validations**:<br />`required`: A boolean to stricly require a value

<a name="multipleSelect"></a>

- ### **multipleSelect**

    Required Properties
    
    Property | Description
    --- | ---
    `controlType` | **multipleSelect**
    `default` | An array of strings to set as the selected value(s) on render in the UI. These values should also appear in the `enum` property.
    `enum` | An array of strings to be used as the dropdown values and labels
    `name` | A string to describe this dropdown. This value is used to manage this dropdown in the UI and should contain only letters, numbers, dashes or underscores.
    `path` | The JSONPath for this dropdown
    `title` | A string to label this dropdown. This value should be a _"Pretty"_ value to display in the UI.

    Optional Properties

    Property | Description
    --- | ---
    `description` | A string to describe this dropdown. This will be displayed directly below the dropdown in the UI.
    `placeholder` | A string to be used as this dropdowns placeholder value.
    `validation` | An array of validation objects that this dropdown must satisfy to be valid. Each object can also contain a `customError` field which is a string to override the default error message shown when that validation criteria fails.<br /><br />**Supported Validations**:<br />`required`: A boolean to stricly require a value

<a name="checkboxSingle"></a>

- ### **checkboxSingle**

    Required Properties
    
    Property | Description
    --- | ---
    `controlType` | **checkboxSingle**
    `name` | A string to describe this checkbox. This value is used to manage this checkbox in the UI and should contain only letters, numbers, dashes or underscores.
    `path` | The JSONPath for this checkbox
    `title` | A string to label this checkbox. This value should be a _"Pretty"_ value to display in the UI.

    Optional Properties

    Property | Description
    --- | ---
    `default` | A boolean to set this checkbox as checked(**true**) or unchecked(**false**) on render in the UI. By default, checkboxes are rendered as unchecked.
    `description` | A string to describe this checkbox. This will be displayed directly below the checkbox in the UI.
    `validation` | An array of validation objects that this checkbox must satisfy to be valid. Each object can also contain a `customError` field which is a string to override the default error message shown when that validation criteria fails.<br /><br />**Supported Validations**:<br />`required`: A boolean to stricly require this checkbox to be checked(**true**)

<a name="checkboxGroup"></a>

- ### **checkboxGroup**

    Required Properties
    
    Property | Description
    --- | ---
    `controlType` | **checkboxGroup**
    `enum` | An array of strings to be used as labels for each checkbox in the group
    `name` | A string to describe this checkbox group. This value is used to manage this checkbox group in the UI and should contain only letters, numbers, dashes or underscores.
    `path` | The JSONPath for this checkbox group
    `title` | A string to label the checkbox group as a whole. This value should be a _"Pretty"_ value to display in the UI.

    Optional Properties

    Property | Description
    --- | ---
    `default` | A map of key/value pairs where each key is a string that also appears in the enum property and its value is a boolean to set that checkbox as checked(**true**) or unchecked(**false**) on render in the UI. By default, all checkboxes in a group are rendered as unchecked.
    `description` | A string to describe this checkbox. This will be displayed directly below the checkbox in the UI.
    `validation` | An array of validation objects that this checkbox must satisfy to be valid. Each object can also contain a `customError` field which is a string to override the default error message shown when that validation criteria fails.<br /><br />**Supported Validations**:<br />`required`: A boolean to stricly require one or more checkboxes in the group to be checked(**true**)

    `default` **Example**:

    ```yaml
    default:
      'Option One': true
      'Option Two': false
      'Option Three': true
    ```

<a name="radioGroup"></a>

- ### **radioGroup**

    Required Properties
    
    Property | Description
    --- | ---
    `controlType` | **radioGroup**
    `enum` | An array of strings to be used as labels for each radio button in the group
    `name` | A string to describe this radio group. This value is used to manage this radio group in the UI and should contain only letters, numbers, dashes or underscores.
    `path` | The JSONPath for this radio group
    `title` | A string to label the radio group as a whole. This value should be a _"Pretty"_ value to display in the UI.

    Optional Properties

    Property | Description
    --- | ---
    `default` | A string to set the radio button with the same label as selected on render in the UI. By default, the first radio button in the group will be selected.
    `description` | A string to describe this radio group. This will be displayed directly below the radio group in the UI.
    `validation` | An array of validation objects that this checkbox must satisfy to be valid. Each object can also contain a `customError` field which is a string to override the default error message shown when that validation criteria fails.<br /><br />**Supported Validations**:<br />`pattern`: A Regular Expression pattern that the selected radio button value must satisfy

<a name="codeSnippet"></a>

- ### **codeSnippet**

    Required Properties
    
    Property | Description
    --- | ---
    `codeText` | The entirety of content you wish to have displayed in the code snippet area as a string.
    `controlType` | **codeSnippet**

    Optional Properties

    Property | Description
    --- | ---
    `description` | A string to describe this code snippet. This will be displayed directly below the code snippet in the UI.

    _**Important Notes**: Currently the code snippet does not support any language formatting or highlighting, however this may be added in the future. For now, do not expect any special treatment given to the string provided in the `codeText` property. What is provided will be exactly what is rendered in the UI within a styled code snippet block._

-----