// Create new database
async function createDatabase() {
    const dbName = document.getElementById('dbName').value;
    if (!dbName) {
        showError('Please enter a database name');
        return;
    }

    try {
        const response = await fetch('/api/database/create', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ db_name: dbName })
        });

        const result = await response.json();
        if (result.status === 'ok') {
            showSuccess(result.message);
        } else {
            showError(result.message);
        }
    } catch (error) {
        showError('Failed to create database: ' + error.message);
    }
}

// Add new column to table creation form
function addColumn() {
    const columnsDiv = document.getElementById('columns');
    const columnDiv = document.createElement('div');
    columnDiv.className = 'column';
    columnDiv.innerHTML = `
        <input type="text" placeholder="Column Name" required>
        <select class="column-type">
            <option value="INT">INT</option>
            <option value="VARCHAR(255)">VARCHAR</option>
            <option value="TEXT">TEXT</option>
            <option value="DATETIME">DATETIME</option>
            <option value="BOOLEAN">BOOLEAN</option>
            <option value="FLOAT">FLOAT</option>
            <option value="DOUBLE">DOUBLE</option>
        </select>
        <label><input type="checkbox" class="nullable"> Nullable</label>
        <label><input type="checkbox" class="primary-key"> Primary Key</label>
        <button onclick="this.parentElement.remove()" class="remove">Remove</button>
    `;
    columnsDiv.appendChild(columnDiv);
    
    // Focus on the new input field
    const newInput = columnDiv.querySelector('input[type="text"]');
    newInput.focus();
}

// Create new table
async function createTable() {
    const dbName = document.getElementById('tableDB').value;
    const tableName = document.getElementById('tableName').value;
    
    if (!dbName || !tableName) {
        showError('Please enter database and table names');
        return;
    }

    const columns = [];
    document.querySelectorAll('.column').forEach(columnDiv => {
        const nameInput = columnDiv.querySelector('input[type="text"]');
        const typeSelect = columnDiv.querySelector('.column-type');
        const nullable = columnDiv.querySelector('.nullable').checked;
        const primaryKey = columnDiv.querySelector('.primary-key').checked;
        
        if (nameInput.value) {
            let columnDef = `${nameInput.value} ${typeSelect.value}`;
            if (!nullable) columnDef += ' NOT NULL';
            if (primaryKey) columnDef += ' PRIMARY KEY';
            columns.push(columnDef);
        }
    });

    if (columns.length === 0) {
        showError('Please add at least one column');
        return;
    }

    const createTableQuery = `CREATE TABLE ${dbName}.${tableName} (${columns.join(', ')})`;

    try {
        const response = await fetch('/api/query', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                query: createTableQuery,
                token: 'secret-token'
            })
        });

        const result = await response.json();
        if (result.status === 'ok') {
            showSuccess(result.message);
        } else {
            showError(result.message);
        }
    } catch (error) {
        showError('Failed to create table: ' + error.message);
    }
}

// Execute SQL query
async function executeQuery() {
    const query = document.getElementById('query').value;
    if (!query) {
        showError('Please enter a query');
        return;
    }

    try {
        const response = await fetch('/api/query', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                query: query,
                token: 'secret-token'
            })
        });

        const result = await response.json();
        if (result.status === 'ok') {
            if (result.rows && result.rows.length > 0) {
                displayResults(result);
            } else {
                showSuccess(result.message);
            }
        } else {
            showError(result.message);
        }
    } catch (error) {
        showError('Failed to execute query: ' + error.message);
    }
}

// Display query results in a table
function displayResults(result) {
    const resultDiv = document.getElementById('queryResult');
    let html = '<table><thead><tr>';
    
    // Add headers
    result.header.forEach(header => {
        html += `<th>${header}</th>`;
    });
    html += '</tr></thead><tbody>';

    // Add rows
    result.rows.forEach(row => {
        html += '<tr>';
        row.forEach(cell => {
            html += `<td>${cell}</td>`;
        });
        html += '</tr>';
    });

    html += '</tbody></table>';
    resultDiv.innerHTML = html;
}

// Show error message
function showError(message) {
    const resultDiv = document.getElementById('queryResult');
    resultDiv.innerHTML = `<div class="error">${message}</div>`;
}

// Show success message
function showSuccess(message) {
    const resultDiv = document.getElementById('queryResult');
    resultDiv.innerHTML = `<div class="success">${message}</div>`;
}

async function loadSlaves() {
    const div = document.getElementById('slavesList');
    div.innerHTML = "Loading...";
    try {
        const res = await fetch('/api/slaves');
        const data = await res.json();
        if (Object.keys(data).length === 0) {
            div.innerHTML = "No slaves connected.";
            return;
        }
        let html = "<table><tr><th>IP</th><th>Last Seen</th></tr>";
        for (const [ip, lastSeen] of Object.entries(data)) {
            html += `<tr><td>${ip}</td><td>${lastSeen}</td></tr>`;
        }
        html += "</table>";
        div.innerHTML = html;
    } catch (e) {
        div.innerHTML = "Error loading slaves.";
    }
}

// Drop database
async function dropDatabase() {
    const dbName = document.getElementById('dropDbName').value;
    if (!dbName) {
        showError('Please enter a database name');
        return;
    }

    if (!confirm(`Are you sure you want to drop database "${dbName}"?`)) {
        return;
    }

    try {
        const response = await fetch('/api/query', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                query: `DROP DATABASE ${dbName}`,
                token: 'secret-token'
            })
        });

        const result = await response.json();
        if (result.status === 'ok') {
            showSuccess(result.message);
        } else {
            showError(result.message);
        }
    } catch (error) {
        showError('Failed to drop database: ' + error.message);
    }
}

// Drop table
async function dropTable() {
    const dbName = document.getElementById('dropTableDB').value;
    const tableName = document.getElementById('dropTableName').value;
    
    if (!dbName || !tableName) {
        showError('Please enter database and table names');
        return;
    }

    if (!confirm(`Are you sure you want to drop table "${tableName}" from database "${dbName}"?`)) {
        return;
    }

    try {
        const response = await fetch('/api/query', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                query: `DROP TABLE ${dbName}.${tableName}`,
                token: 'secret-token'
            })
        });

        const result = await response.json();
        if (result.status === 'ok') {
            showSuccess(result.message);
        } else {
            showError(result.message);
        }
    } catch (error) {
        showError('Failed to drop table: ' + error.message);
    }
}

// Data Operations
function addInsertField() {
    const fieldsDiv = document.getElementById('insertFields');
    const fieldDiv = document.createElement('div');
    fieldDiv.className = 'insert-field';
    fieldDiv.innerHTML = `
        <input type="text" placeholder="Column Name">
        <input type="text" placeholder="Value">
        <button onclick="this.parentElement.remove()" class="remove">Remove</button>
    `;
    fieldsDiv.appendChild(fieldDiv);
}

async function insertData() {
    const dbName = document.getElementById('insertDB').value;
    const tableName = document.getElementById('insertTable').value;
    
    if (!dbName || !tableName) {
        showError('Please enter database and table names');
        return;
    }

    const columns = [];
    const values = [];
    document.querySelectorAll('.insert-field').forEach(fieldDiv => {
        const inputs = fieldDiv.querySelectorAll('input');
        if (inputs[0].value && inputs[1].value) {
            columns.push(inputs[0].value);
            values.push(inputs[1].value);
        }
    });

    if (columns.length === 0) {
        showError('Please add at least one field');
        return;
    }

    const insertQuery = `INSERT INTO ${dbName}.${tableName} (${columns.join(', ')}) VALUES (${values.map(v => `'${v}'`).join(', ')})`;

    try {
        const response = await fetch('/api/query', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                query: insertQuery,
                token: 'secret-token'
            })
        });

        const result = await response.json();
        if (result.status === 'ok') {
            showSuccess(result.message);
        } else {
            showError(result.message);
        }
    } catch (error) {
        showError('Failed to insert data: ' + error.message);
    }
}

function addWhereCondition() {
    const conditionsDiv = document.getElementById('whereConditions');
    const conditionDiv = document.createElement('div');
    conditionDiv.className = 'condition';
    conditionDiv.innerHTML = `
        <input type="text" placeholder="Column">
        <select>
            <option value="=">=</option>
            <option value=">">></option>
            <option value="<"><</option>
            <option value=">=">>=</option>
            <option value="<="><=</option>
            <option value="!=">!=</option>
            <option value="LIKE">LIKE</option>
        </select>
        <input type="text" placeholder="Value">
        <button onclick="this.parentElement.remove()" class="remove">Remove</button>
    `;
    conditionsDiv.appendChild(conditionDiv);
}

async function selectData() {
    const dbName = document.getElementById('selectDB').value;
    const tableName = document.getElementById('selectTable').value;
    
    if (!dbName || !tableName) {
        showError('Please enter database and table names');
        return;
    }

    let selectQuery = 'SELECT ';
    
    // Handle columns selection
    if (document.getElementById('selectAll').checked) {
        selectQuery += '*';
    } else {
        const selectedColumns = Array.from(document.querySelectorAll('.select-column input:checked'))
            .map(input => input.value);
        if (selectedColumns.length === 0) {
            showError('Please select at least one column');
            return;
        }
        selectQuery += selectedColumns.join(', ');
    }

    selectQuery += ` FROM ${dbName}.${tableName}`;

    // Handle WHERE conditions
    const conditions = [];
    document.querySelectorAll('.condition').forEach(conditionDiv => {
        const inputs = conditionDiv.querySelectorAll('input');
        const operator = conditionDiv.querySelector('select').value;
        if (inputs[0].value && inputs[1].value) {
            conditions.push(`${inputs[0].value} ${operator} '${inputs[1].value}'`);
        }
    });

    if (conditions.length > 0) {
        selectQuery += ' WHERE ' + conditions.join(' AND ');
    }

    try {
        const response = await fetch('/api/query', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                query: selectQuery,
                token: 'secret-token'
            })
        });

        const result = await response.json();
        if (result.status === 'ok') {
            if (result.rows && result.rows.length > 0) {
                displayResults(result);
            } else {
                showSuccess('No results found');
            }
        } else {
            showError(result.message);
        }
    } catch (error) {
        showError('Failed to execute select: ' + error.message);
    }
}

async function deleteColumn() {
    const dbName = document.getElementById('deleteColumnDB').value;
    const tableName = document.getElementById('deleteColumnTable').value;
    const columnName = document.getElementById('deleteColumnName').value;
    if (!dbName || !tableName || !columnName) {
        showError("Please enter Database Name, Table Name, and Column Name.");
        return;
    }
    const query = `ALTER TABLE ${dbName}.${tableName} DROP COLUMN ${columnName}`;
    try {
        const response = await fetch('/api/query', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ query, token: 'secret-token' }) });
        const result = await response.json();
        if (result.status === 'ok') {
            showSuccess("Column deleted successfully.");
        } else {
            showError("Error deleting column: " + (result.message || "Unknown error"));
        }
    } catch (e) {
        showError("Error executing delete column query: " + e.message);
    }
}

async function updateData() {
    const dbName = document.getElementById('updateDB').value;
    const tableName = document.getElementById('updateTable').value;
    if (!dbName || !tableName) {
        showError("Please enter Database Name and Table Name.");
        return;
    }
    const updateFields = [];
    document.querySelectorAll('#updateFields .update-field').forEach(field => {
        const col = field.querySelector('input[placeholder="Column"]').value;
        const val = field.querySelector('input[placeholder="Value"]').value;
        if (col && val) {
            updateFields.push(`${col} = '${val}'`);
        }
    });
    if (updateFields.length === 0) {
        showError("Please add at least one update field.");
        return;
    }
    const whereConditions = [];
    document.querySelectorAll('#updateWhereConditions .condition').forEach(cond => {
        const col = cond.querySelector('input[placeholder="Column"]').value;
        const op = cond.querySelector('select').value;
        const val = cond.querySelector('input[placeholder="Value"]').value;
        if (col && val) {
            whereConditions.push(`${col} ${op} '${val}'`);
        }
    });
    let query = `UPDATE ${dbName}.${tableName} SET ${updateFields.join(', ')}`;
    if (whereConditions.length > 0) {
        query += " WHERE " + whereConditions.join(' AND ');
    }
    try {
        const response = await fetch('/api/query', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ query, token: 'secret-token' }) });
        const result = await response.json();
        if (result.status === 'ok') {
            showSuccess("Update executed successfully.");
        } else {
            showError("Error executing update query: " + (result.message || "Unknown error"));
        }
    } catch (e) {
        showError("Error executing update query: " + e.message);
    }
}

function addUpdateField() {
    const fieldsDiv = document.getElementById('updateFields');
    const fieldDiv = document.createElement('div');
    fieldDiv.className = 'update-field';
    fieldDiv.innerHTML = `
        <input type="text" placeholder="Column">
        <input type="text" placeholder="Value">
        <button onclick="this.parentElement.remove()" class="remove">Remove</button>
    `;
    fieldsDiv.appendChild(fieldDiv);
} 