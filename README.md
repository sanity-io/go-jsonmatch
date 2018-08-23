# JSONMatch

JSONMatch is a variant of JSONPath that simplifes the syntax and eliminating to the maximum extent the number of special characters required to express a path.

The result of matching a document using JSONMatch is a set of matched values in the document. The present library provides functions for mutating, setting or deleting the values in the match set.

## Examples

Select the first name:

`name.first`

Select all keys (or array values) in the designated path:

`employees.tasks.*` or `employee.tasks[*]`

Select the first and second name (union)

`name[first, second]`

Select an employee from an array based on index

`employees[5]`

Select a subset of employees from an array according to index

`employees[1,2,5,9]`

Select employees 1 through 3 using the slice operation

`employees[1:3]`

Union of two ranges of employees

`employees[1:3, 9:12]`

Union of a range and a number of individual indicies

`employees[1:3, 9, 12]`

Select a subset of employees using a filter

`employees[wage > 50000]`

Select a subset of employees filtering based on the existence of the key
'bonus'.

`employees[bonus?]`

Recursively select any sub-document under 'some.path' in the document matching the filter:

`some.path..[key=="4f5xa"]`

Select number from an array where the individual numbers match the filter. (@ == this)

`numbers[@ > 50]`

Union of completely separate paths:

`[employee[5].name, company.name, wageTiers[compensation > 10000]]`

Enclose attribute names in single quotes (where needed), and literal strings in double quotes:

`employees['the name' == "John Smith"]``

Currently filters in jsonpath2 do not support boolean operations, although `,` is synonymous with `or`/`||`. You could do:

`employees[name == "John Smith", name == "Granny Smith"]``

There is no way to express an intersection. The following is invalid syntax, but considered for a future version:

`INVALID: employees[name.first == "John" && name.last == "Smith"]`

[NOTE: Apparently there is a way to do intersections with the current syntax. You can actually do this:
`employees[name.first == "John"][name.last == "Smith"]`. It looks weird, but it works.]

Select from start of array:

`array[:3]`

Select through end of array:

`array[3:]`

Select every element in array:

`array[*]` (why not just `:`, i don't know, ask jsonpath1)

Select the last element of an array:

`array[-1]`

## Acknowledgements

The code was originally forked from the Kubernetes JSONPath parser. However, it has since been heavily modified.
