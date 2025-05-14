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

// Search records
async function searchRecords() {
    const dbName = document.getElementById('searchDB').value;
    const tableName = document.getElementById('searchTable').value;
    const column = document.getElementById('searchColumn').value;
    const value = document.getElementById('searchValue').value;

    if (!dbName || !tableName) {
        showError('Please enter database and table names');
        return;
    }

    let query = `SELECT * FROM ${dbName}.${tableName}`;
    if (column && value) {
        query += ` WHERE ${column} LIKE '%${value}%'`;
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
                showSuccess('No records found');
            }
        } else {
            showError(result.message);
        }
    } catch (error) {
        showError('Failed to search records: ' + error.message);
    }
}

// Update records
async function updateRecords() {
    const dbName = document.getElementById('updateDB').value;
    const tableName = document.getElementById('updateTable').value;
    const column = document.getElementById('updateColumn').value;
    const value = document.getElementById('updateValue').value;
    const where = document.getElementById('updateWhere').value;

    if (!dbName || !tableName || !column || !value || !where) {
        showError('Please fill all fields');
        return;
    }

    const query = `UPDATE ${dbName}.${tableName} SET ${column} = '${value}' WHERE ${where}`;

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
            showSuccess(result.message);
        } else {
            showError(result.message);
        }
    } catch (error) {
        showError('Failed to update records: ' + error.message);
    }
}

// Delete records
async function deleteRecords() {
    const dbName = document.getElementById('deleteDB').value;
    const tableName = document.getElementById('deleteTable').value;
    const where = document.getElementById('deleteWhere').value;

    if (!dbName || !tableName || !where) {
        showError('Please fill all fields');
        return;
    }

    const query = `DELETE FROM ${dbName}.${tableName} WHERE ${where}`;

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
            showSuccess(result.message);
        } else {
            showError(result.message);
        }
    } catch (error) {
        showError('Failed to delete records: ' + error.message);
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